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
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css" integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm" crossorigin="anonymous">

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
    <div class="text-center srv">xSyn</div>

    <div class="row">

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

    </div>

    <!-- bootstrap -->
    <script src="https://code.jquery.com/jquery-3.2.1.slim.min.js" integrity="sha384-KJ3o2DKtIkvYIK3UENzmM7KCkRr/rE9/Qpg6aAZGJwFDMVNA/GpGFF93hXpG5KkN" crossorigin="anonymous"></script>
    <script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js" integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl" crossorigin="anonymous"></script>

    </script>
</body>

</html>`
