import http.server
import socketserver
import os

# Usamos el puerto 8000 por defecto, o el que se pase por la variable de entorno PORT
PORT = int(os.environ.get("PORT", 8000))
Handler = http.server.SimpleHTTPRequestHandler

# Cambiamos al directorio donde está el script para servir los archivos desde ahí
os.chdir(os.path.dirname(os.path.abspath(__file__)))

with socketserver.TCPServer(("", PORT), Handler) as httpd:
    print(f"Sirviendo aplicación web en el puerto {PORT}")
    httpd.serve_forever()
