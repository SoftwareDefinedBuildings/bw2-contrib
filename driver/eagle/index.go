package main

import (
	"html/template"
)

var _INDEX = []byte(`
<html>
  <head>
	  <meta charset="utf-8">
  	  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.98.2/css/materialize.min.css">
	  <script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.98.2/js/materialize.min.js"></script>
  </head>
  <body>
  	<h1>EAGLE</h1>
	<div class="container">
		<div class="row">
			<form action="/login" method="post">
				<div class="col s6 offset-s3">
					<label><b>Username</b></label>
					<input type="text" placeholder="Enter Username" name="user" required>

					<label><b>Password</b></label>
					<input type="password" placeholder="Enter Password" name="pass" required>

					<button type="submit">Login</button>
				</div>
			</form>
		</div>
	</div>
  </body>
</html>
`)

var _CONFIG = []byte(`
<html>
  <head>
	  <meta charset="utf-8">
  	  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.98.2/css/materialize.min.css">
	  <script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.98.2/js/materialize.min.js"></script>
  </head>
  <body>
  	<h1>EAGLE</h1>
	<div class="container">
		<div class="row">
			<form action="/config" method="post">
				<div class="col s6 offset-s3">
					<label><b>BASE URI</b></label>
					<input type="text" placeholder="scratch.ns/devices" name="baseuri" required>

					<button type="submit">Get URL</button>
				</div>
			</form>
  		</div>
	</div>
  </body>
</html>
`)

var _RESULT = template.Must(template.New("result").Parse(`
<html>
  <head>
	  <meta charset="utf-8">
  	  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.98.2/css/materialize.min.css">
	  <script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.98.2/js/materialize.min.js"></script>
  </head>
  <body>
  	<h1>EAGLE</h1>
	<div class="container">
		<ul class="collection">
			<li class="collection-item"><b>Base URI: </b>{{.baseuri}}</li>
			<li class="collection-item"><b>Hash: </b>{{.hash}}</li>
			<li class="collection-item"><b>Eagle Report URL: </b>{{.reporturl}}</li>
		</ul>
		{{if .error}}
		<div class="row">
			<p>ERROR? <b>{{.error}}</b></p>
		</div>
		{{else}}
		{{end}}
	</div>
  </body>
</html>
`))
