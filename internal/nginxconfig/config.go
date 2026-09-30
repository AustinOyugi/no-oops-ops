// Package nginxconfig builds the single configuration used by managed nginx.
package nginxconfig

const ContainerPath = "/etc/noops/nginx/nginx.conf"

func Wrap(httpConfig []byte) []byte {
	return append(append([]byte("user nginx;\nworker_processes auto;\npid /var/run/nginx.pid;\nerror_log /var/log/nginx/error.log notice;\nevents { worker_connections 1024; }\nhttp {\n  include /etc/nginx/mime.types;\n  default_type application/octet-stream;\n  access_log /var/log/nginx/access.log;\n  sendfile on;\n  keepalive_timeout 65;\n"), httpConfig...), []byte("\n}\n")...)
}
