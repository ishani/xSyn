package main

var frontpageHTML = `
<!DOCTYPE html>
<html lang="en">

<head>
    <!-- Required meta tags -->
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>xSyn | Custom xBrowserSync Server</title>

    <!-- bootstrap -->
    <link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/css/bootstrap.min.css" integrity="sha384-MCw98/SFnGE8fJT3GXwEOngsV7Zt27NXFoaoApmYm81iuXoPkFOJwJ8ERdknLPMO" crossorigin="anonymous">

    <link href="https://fonts.googleapis.com/css?family=Open+Sans:400|Abel" rel="stylesheet">
    <style type="text/css">

    body {
        background-color: #202d50;
        margin-top: 1rem;

        font-family: 'Open Sans', sans-serif;
        font-size: 16px;
    }
    .srv {
        color: white;

        font-family: 'Abel', sans-serif;
        font-size: 64px;
        font-weight: 400;
        padding: 2rem;
    }

    </style>

</head>

<body>
    <div class="container">
        <div class="jumbotron shadow p-3 mb-5">
        <h1 class="display-4">xSyn</h1>
        <p class="lead">
        A fast, compact and <i>Docker</i>able server for <a href="https://www.xbrowsersync.org/" target="_blank">xBrowserSync</a>, written in Go, backed by <a href="https://github.com/boltdb/bolt" target="_blank">BoltDB</a>
        </p>
        <hr class="my-4">
        <p>Written by Harry Denholm, ishani.org 2018</p>
        <a class="btn btn-primary btn-lg" href="https://github.com/ishani/xSyn" target="_blank">Source on GitHub</a>
    </div>

    <div class="row">
    {{ if not . }}
        <div class="col-12 pb-5">
            <div class="card-header bg-info text-white shadow rounded">
                <p id="vcode">[/info]</p>
            </div>
        </div>
    {{ else }}
        {{ range $key, $value := . }}
            <div class="col-4 pb-5">
                <div class="card border-dark">
                    <div class="card-header bg-primary text-white">
                        {{ $key }}
                    </div>
                    <ul class="list-group list-group-flush">
                    {{ range $sub_key, $sub_value := $value }}
                        <li class="list-group-item d-flex justify-content-between align-items-center">
                        {{ $sub_key }}
                        <span class="badge badge-primary badge-pill">{{ $sub_value }}</span>
                        </li>
                    {{ end }}
                    </ul>
                </div>
            </div>
        {{ end }}
    {{ end }}
    </div>

    <!-- bootstrap -->
    <script src="https://ajax.aspnetcdn.com/ajax/jQuery/jquery-3.3.1.min.js" ></script>
    <script src="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/js/bootstrap.min.js" integrity="sha384-ChfqqxuZUCnJSK3+MXmPNIyE6ZbWh2IMqE241rYiqJxyMiZ6OW/JmZQ5stwEULTy" crossorigin="anonymous"></script>

    <script>

    $.when(
      $.getJSON( "/info" ),
      $.ready
    ).done(function( data ) {
        var items = [];
        $.each( data[0], function( key, val ) {
          items.push( "<li> " + key + " = " + val + "</li>" );
        });
     
        $( "<ul/>", {
          "class": "my-new-list",
          html: items.join( "" )
        }).appendTo( "p#vcode" );
    });

    </script>
</body>

</html>`
