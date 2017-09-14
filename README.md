Command pdfsvc is a small wrapper around wkhtmltopdf command to expose it as
a http service.

Service accepts POST requests expecting utf8 html bodies and proper
`Content-Type: text/html` header. If html is successfully converted, reply
would have code 200 OK and `Content-Type: application/pdf`, the body would have
pdf document.

If pdfsvc is started with `TOKEN` environment variable or `-token=value` flag,
only requests having `Authorization: Bearer token` header are allowed.
