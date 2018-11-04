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
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/fatih/structs"
	"github.com/gin-contrib/size"
	"github.com/gin-gonic/autotls"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
)

// tbd; make the Debug/Prod choice here configurable
var zLog, _ = zap.NewProduction()

// BuildStamp can be written to externally during a go build to apply a build-time string, like a timestamp
var BuildStamp string = "[unstamped]"

// names for buckets where we hide our data
var boltDataBucket = []byte("BM")
var boltTimestampBucket = []byte("TS")
var boltVersionBucket = []byte("VR")

// CreateBookmarkData is received in POST /bookmarks
type CreateBookmarkData struct {
	ClientVersion string `json:"version"`
}

// RequestData is received in the POST and PUT methods
type RequestData struct {
	EncodedBookmarks string `json:"bookmarks"`
}

// by default we accept new sync IDs - ie. new users for the service;
// this can be overridden in the config and toggled live, if required
var newSyncsAllowed = true

func synAcceptTOS(tosURL string) bool {
	zLog.Info("Autocert TOS", zap.String("URL", tosURL))
	return true
}

func main() {

	// fetch config from toml, apply env overrides, etc
	LoadConfig()

	// log out the build stamp and record when we booted up to show on /stats
	// helps me ensure that webhooks et al are firing and servers are up to date as expected
	zLog.Info("xSyn", zap.String("Build", BuildStamp))
	bootTime := time.Now().UTC()

	// if a cache path was given for LetsEncrypt, trial-run the creation of it
	// so we know early on that the storage has been configured correctly
	if len(AppConfig.Security.LetsEncryptCache) > 0 {
		if err := os.MkdirAll(AppConfig.Security.LetsEncryptCache, 0700); err != nil {
			zLog.Panic("LE cache path test",
				zap.String("path", AppConfig.Security.LetsEncryptCache),
				zap.Error(err),
			)
		}
	}

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

	// ensure the bucket collection exists, create them if not
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(boltDataBucket)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists(boltTimestampBucket)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists(boltVersionBucket)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		zLog.Panic("Bucket creation", zap.Error(err))
	}

	db.Sync()
	if _, err := os.Stat(AppConfig.Bolt.StorageFile); os.IsNotExist(err) {
		zLog.Panic("BoltDB file check", zap.Error(err))
	}

	// switch to release?
	if AppConfig.Server.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	// build a Gin instance with default middleware
	router := gin.Default()

	// apply rate limiting middleware if specified
	if AppConfig.Security.ReqPerSecond > 0 {

		zLog.Info("Adding rate-limiting", zap.Float64("RPS", AppConfig.Security.ReqPerSecond))

		// I've chosen a fairly arbitrary burst limit to allow XBS to poll a few things during a sync without
		// exhausting the limits immediately as this limit is applied to all routes
		limiter := tollbooth.NewLimiter(AppConfig.Security.ReqPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
		limiter.SetBurst(20)
		router.Use(tollbooth_gin.LimitHandler(limiter))
	}

	// magic route to toggle new-sync option
	if len(AppConfig.Security.SyncToggleRoute) > 0 {

		zLog.Info("Enabling sync toggling route")

		router.GET(AppConfig.Security.SyncToggleRoute, func(c *gin.Context) {
			newSyncsAllowed = !newSyncsAllowed
			c.String(200, fmt.Sprintf("Toggled accept_new_syncs to [%t]", newSyncsAllowed))
		})
	}

	// route to create a new sync ID
	router.POST("/bookmarks", func(c *gin.Context) {

		// sorry, we're closed for business
		if newSyncsAllowed == false {
			c.JSON(409, gin.H{
				"code":    "NotAllowed",
				"message": "Not accepting new sync users",
			})
			return
		}

		var bookmarkData CreateBookmarkData
		if err := c.ShouldBindJSON(&bookmarkData); err != nil {
			handleError(c, "MissingParameter", "/bookmarks POST missing", err)
			return
		}

		zLog.Debug("New SyncID requested", zap.String("Client", bookmarkData.ClientVersion))

		newID := "invalid"
		imprintTime := createTimestampString()

		err = db.Update(func(tx *bolt.Tx) error {

			bkData := tx.Bucket(boltDataBucket)
			bkTs := tx.Bucket(boltTimestampBucket)
			bkVer := tx.Bucket(boltVersionBucket)

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
				zLog.Warn("Duplicate UUID, retrying", zap.Int("Count", uniqueIDRetryCount))

				// .. so do that
				if uniqueIDRetryCount > 8 {
					return fmt.Errorf("too many UUID collisions")
				}
			}

			// copy out the ID
			newID = string(buf)

			if err = bkData.Put(buf, make([]byte, 0)); err != nil {
				return err
			}

			if err = bkVer.Put(buf, []byte(bookmarkData.ClientVersion)); err != nil {
				return err
			}

			err = bkTs.Put(buf, []byte(imprintTime))
			return err
		})

		if handleError(c, "InternalError", "", err) {
			return
		}

		zLog.Debug("New key created", zap.String("key", newID))

		c.JSON(200, gin.H{
			"id":          newID,
			"lastUpdated": imprintTime,
			"version":     bookmarkData.ClientVersion,
		})
	})

	// fetch the bookmarks data for the given SyncID
	router.GET("/bookmarks/:id", func(c *gin.Context) {
		markID := c.Param("id")
		markIDBytes := []byte(markID)

		var dataResult string
		var tsResult string
		var verResult string

		err := db.View(func(tx *bolt.Tx) error {

			bkData := tx.Bucket(boltDataBucket)
			bkTs := tx.Bucket(boltTimestampBucket)
			bkVer := tx.Bucket(boltVersionBucket)

			data := bkData.Get(markIDBytes)
			ts := bkTs.Get(markIDBytes)
			ver := bkVer.Get(markIDBytes)

			if data == nil {
				return errors.New("data not found for key")
			}
			if ts == nil {
				return errors.New("timestamp not found for key")
			}
			if ver == nil {
				return errors.New("version not found for key")
			}

			// copy out
			dataResult = string(data)
			tsResult = string(ts)
			verResult = string(ver)

			return nil
		})

		if handleError(c, "InvalidArgument", "Invalid ID", err) {
			return
		}

		c.JSON(200, gin.H{
			"bookmarks":   dataResult,
			"lastUpdated": tsResult,
			"version":     verResult,
		})
	})

	maxSyncSizeBytes := int64(1024 * AppConfig.Server.MaxSyncSizeKb)

	sizeLimitedRoutes := router.Group("/", limits.RequestSizeLimiter(maxSyncSizeBytes))
	{
		// replace bookmarks data for the given SyncID
		sizeLimitedRoutes.PUT("/bookmarks/:id", func(c *gin.Context) {
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
	}

	// return the timestamp of the last update for the given SyncID
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

	// return the client version used to create the SyncID
	router.GET("/bookmarks/:id/version", func(c *gin.Context) {
		markID := c.Param("id")
		markIDBytes := []byte(markID)

		var versionString string
		err := db.View(func(tx *bolt.Tx) error {

			bkVer := tx.Bucket(boltVersionBucket)

			versionString = string(bkVer.Get(markIDBytes))
			return nil
		})

		if handleError(c, "InternalError", "", err) {
			return
		}

		if len(versionString) > 0 {
			c.JSON(200, gin.H{
				"version": versionString,
			})
			return
		}

		// return empty json table to signal 'not found'
		c.String(200, "{}")
	})

	router.GET("/info", func(c *gin.Context) {

		serviceStatus := 1
		if newSyncsAllowed == false {
			serviceStatus = 3
		}

		c.JSON(200, gin.H{
			"status":      serviceStatus,
			"message":     AppConfig.Server.ServiceMessage,
			"version":     "1.1.5",
			"buildstamp":  BuildStamp,
			"maxSyncSize": maxSyncSizeBytes,
		})
	})

	// show a basic front page
	// .. passing in nil for the data means we don't show any statistics
	router.GET("/", func(c *gin.Context) {

		t := template.New("frontpage")
		t, _ = t.Parse(frontpageHTML)

		// stream out the execution
		c.Status(200)
		c.Stream(func(w io.Writer) bool {
			t.Execute(w, nil)
			return false
		})
	})

	// .. unlike for this route, which shows the front page but
	// also a bunch of internal stats from BoltDB; the URL for this page
	// can be set in config to something obfuscated if desired
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
		dbstat["build stamp"] = BuildStamp
		dbstat["boot time"] = bootTime.Format(time.RFC850)
		datamap["State"] = dbstat

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

	if len(AppConfig.Security.TLSCert) > 0 {

		zLog.Info("Starting server", zap.String("mode", "https"))

		zLog.Fatal("exited", zap.Error(router.RunTLS(
			launchString,
			fmt.Sprintf("%s.pem", AppConfig.Security.TLSCert),
			fmt.Sprintf("%s.key", AppConfig.Security.TLSCert),
		)))

	} else if len(AppConfig.Security.UseLetsEncrypt) > 0 {

		zLog.Info("Starting server", zap.String("mode", "https-lets-encrypt"))

		autocertmgr := autocert.Manager{
			Prompt:     synAcceptTOS,
			HostPolicy: autocert.HostWhitelist(AppConfig.Security.UseLetsEncrypt),
			Cache:      autocert.DirCache(AppConfig.Security.LetsEncryptCache),
		}

		zLog.Fatal("exited", zap.Error(autotls.RunWithManager(router, &autocertmgr)))

	} else {

		zLog.Info("Starting server", zap.String("mode", "http"))

		zLog.Fatal("exited", zap.Error(router.Run(launchString)))
	}
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
