package main

/* xSyn, a compact server implementing the xBrowserSync API;
 * providing an easy way to spin up a self-contained private bookmark sync
 * storage service.
 *
 * harry denholm, 2018; ishani.org
 *
 * https://www.xbrowsersync.org/ for the plugins
 *
 * BoltDB for storing
 * Gin for serving
 * Zap for logging
 * Toml for configuring
 */

import (
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"time"

	"github.com/boltdb/bolt"
	"github.com/fatih/structs"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
)

var zLog, _ = zap.NewProduction()

// names for buckets where we hide our data
var boltDataBucket = []byte("BM")
var boltTimestampBucket = []byte("TS")

func main() {

	// fetch config from toml, apply env overrides, etc
	LoadConfig()

	// open or create the Bolt DB storage file
	db, err := bolt.Open(
		AppConfig.Bolt.StorageFile,
		0600,
		&bolt.Options{Timeout: time.Second * time.Duration(AppConfig.Bolt.InitTimeout)},
	)
	if err != nil {
		zLog.Panic("BoltDB init", zap.Error(err))
	}
	defer db.Close()

	// ensure the bucket collection exists
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(boltDataBucket)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists(boltTimestampBucket)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		zLog.Panic("Bucket creation", zap.Error(err))
	}

	// switch to release
	if AppConfig.Server.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	// line up the routes
	router := gin.Default()
	router.POST("/bookmarks", func(c *gin.Context) {

		var bookmarkData RequestData
		if err := c.ShouldBindJSON(&bookmarkData); err != nil {
			handleError(c, "MissingParameter", "No bookmarks provided", err)
			return
		}

		newID := "invalid"
		imprintTime := createTimestampString()

		err = db.Update(func(tx *bolt.Tx) error {

			bkData := tx.Bucket(boltDataBucket)
			bkTs := tx.Bucket(boltTimestampBucket)

			// fetch a new ID from the bucket
			seqID, _ := bkData.NextSequence()

			// we loop until we generate a unique new ID; although
			// in the best case this loop will usually only run once as
			// the UUIDs should be pretty unique
			buf := make([]byte, 32)
			uniqueIDFound := false
			uniqueIDRetryCount := 0
			for !uniqueIDFound {

				// create a UUID from timestamp
				uuid1, err := uuid.NewV4()
				if err != nil {
					return err
				}

				// mix it with the sequence ID
				uuid2 := uuid.NewV5(uuid1, fmt.Sprintf("%x", seqID))

				// take a slice of the result; xbs wants 32 char ID
				hex.Encode(buf, uuid2[0:16])

				// used yet? if nil, then no, so use it
				existingKey := bkData.Get(buf)
				if existingKey == nil {
					break
				}

				// will loop forever, paranoia suggests we should have
				// a counter and terminate after N runs
				uniqueIDRetryCount++
				zLog.Info("Duplicate UUID, retrying", zap.Int("Count", uniqueIDRetryCount))
			}

			// copy out the ID
			newID = string(buf)

			if err = bkData.Put(buf, []byte(bookmarkData.EncodedBookmarks)); err != nil {
				return err
			}

			err = bkTs.Put(buf, []byte(imprintTime))
			return err
		})

		if handleError(c, "InternalError", "", err) {
			return
		}

		zLog.Info("New key created", zap.String("key", newID))

		c.JSON(200, gin.H{
			"id":          newID,
			"lastUpdated": imprintTime,
		})
	})

	router.GET("/bookmarks/:id", func(c *gin.Context) {
		markID := c.Param("id")
		markIDBytes := []byte(markID)

		var dataResult string
		var tsResult string
		err := db.View(func(tx *bolt.Tx) error {

			bkData := tx.Bucket(boltDataBucket)
			bkTs := tx.Bucket(boltTimestampBucket)

			data := bkData.Get(markIDBytes)
			ts := bkTs.Get(markIDBytes)

			if data == nil {
				return errors.New("data not found for key")
			}
			if ts == nil {
				return errors.New("timestamp not found for key")
			}

			// copy out
			dataResult = string(data)
			tsResult = string(ts)

			return nil
		})

		if handleError(c, "InvalidArgument", "Invalid ID", err) {
			return
		}

		c.JSON(200, gin.H{
			"bookmarks":   dataResult,
			"lastUpdated": tsResult,
		})
	})

	router.PUT("/bookmarks/:id", func(c *gin.Context) {
		markID := c.Param("id")
		markIDBytes := []byte(markID)

		var bookmarkData RequestData
		if err := c.ShouldBindJSON(&bookmarkData); err != nil {
			handleError(c, "MissingParameter", "No bookmarks provided", err)
			return
		}

		imprintTime := createTimestampString()

		err = db.Update(func(tx *bolt.Tx) error {

			bkData := tx.Bucket(boltDataBucket)

			if err = bkData.Put(markIDBytes, []byte(bookmarkData.EncodedBookmarks)); err != nil {
				return err
			}

			bkTs := tx.Bucket(boltTimestampBucket)

			err = bkTs.Put(markIDBytes, []byte(imprintTime))
			return err
		})

		if handleError(c, "InternalError", "", err) {
			return
		}

		c.JSON(200, gin.H{
			"lastUpdated": imprintTime,
		})
	})

	router.GET("/bookmarks/:id/lastUpdated", func(c *gin.Context) {
		markID := c.Param("id")
		markIDBytes := []byte(markID)

		var timestampString string
		err := db.View(func(tx *bolt.Tx) error {

			bkTs := tx.Bucket(boltTimestampBucket)

			timestampString = string(bkTs.Get(markIDBytes))
			return nil
		})

		if handleError(c, "InternalError", "", err) {
			return
		}

		if len(timestampString) > 0 {
			c.JSON(200, gin.H{
				"lastUpdated": timestampString,
			})
			return
		}

		// return empty json table to signal 'not found'
		c.String(200, "{}")
	})

	router.GET("/info", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":      1,
			"message":     AppConfig.Server.ServiceMessage,
			"version":     "1.0.0",
			"maxSyncSize": 1024 * AppConfig.Server.MaxSyncSizeKb,
		})
	})

	router.GET(AppConfig.Server.StatusRoute, func(c *gin.Context) {

		// snag the bolt stats; break the TxStats map out
		// because the template formatter is only expecting 2 levels of iteration
		stats := structs.Map(db.Stats())
		txStats := stats["TxStats"]
		stats["TxStats"] = "..."

		// get some more bits via transaction
		var keyCount int
		var dbSize int64
		_ = db.View(func(tx *bolt.Tx) error {

			bkData := tx.Bucket(boltDataBucket)
			keyCount = bkData.Stats().KeyN
			dbSize = tx.Size()

			return nil
		})

		// top level holder of key->data
		datamap := make(map[string]interface{})

		// pop in the misc stat fragments
		dbstat := make(map[string]interface{})
		dbstat["key count"] = keyCount
		dbstat["db size (bytes)"] = dbSize
		datamap["Data Bucket"] = dbstat

		// .. and then the other maps extracted from bolt
		datamap["Bolt-Db"] = stats
		datamap["Bolt-TxStats"] = txStats

		// parse the template
		t := template.New("frontpage")
		t, _ = t.Parse(frontpageHTML)

		// stream out the execution
		c.Status(200)
		c.Stream(func(w io.Writer) bool {
			t.Execute(w, datamap)
			return false
		})
	})

	launchString := fmt.Sprintf(":%d", AppConfig.Server.Port)
	router.Run(launchString)
}

// RequestData is received in the POST and PUT methods
type RequestData struct {
	EncodedBookmarks string `json:"bookmarks"`
}

// xbs seems to want a 409 when things go wrong; this is a simple wrapper to generate
// the appropriate response, log the underlying Go error and return true if the route handler
// should abort
func handleError(c *gin.Context, code, message string, err error) bool {
	if err != nil {

		if len(message) == 0 {
			message = err.Error()
		}

		c.JSON(409, gin.H{
			"code":    code,
			"message": message,
		})
		zLog.Warn(code, zap.Error(err))
		return true
	}
	return false
}

// xbs expects timestamp in "2016-07-06T12:43:16.866Z" format
func createTimestampString() string {
	return time.Now().Format(time.RFC3339)
}
