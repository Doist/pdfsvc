Command pdfsvc is a small wrapper around [WeasyPrint command][1] to expose it as
a http service.

[1]: https://doc.courtbouillon.org/weasyprint/stable/first_steps.html#command-line

Service accepts POST requests expecting html bodies and proper `Content-Type:
text/html` header. If html is not utf8, either set proper encoding in
`Content-Type` header or directly in html. If html is successfully converted,
reply would have code 200 OK and `Content-Type: application/pdf`, the body
would be a pdf document.

If pdfsvc is started with `TOKEN` environment variable or `-token=value` flag,
only requests having `Authorization: Bearer token` header are allowed.

You can build ready-to-use docker image using Dockerfile from this repository
(Docker 17.05 or later is required):

	docker build -t pdfsvc:latest .

Then run it:

	docker run -p 8080:8080 --rm pdfsvc

You can use `ADDR` environment variable to change address service listens at
and `TOKEN` to enable request authentication.

Example of calling service listening on localhost:8080 with curl:

	curl -sD- -o output.pdf -T input.html \
		-X POST -H "Content-Type: text/html" http://localhost:8080/
