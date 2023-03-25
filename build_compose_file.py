import sys

START = '''version: '3.9'
name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
      - LOGGING_LEVEL=DEBUG
    networks:
      - testing_net

'''

END = '''networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
'''

TO_REPLACE = '''  client<CLIENT_ID>:
    container_name: client<CLIENT_ID>
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID=<CLIENT_ID>
      - CLI_LOG_LEVEL=DEBUG
    networks:
      - testing_net
    depends_on:
      - server

'''

total_clients = 1

try:
    total_clients = int(sys.argv[1])
except ValueError:
    raise ValueError("Argument CLIENT could not be parsed to int.")

with open("docker-compose-dev.yaml", "w") as file:
    content = START
    for client_id in range(1, total_clients + 1):
        content += TO_REPLACE.replace('<CLIENT_ID>', str(client_id))

    content += END
    file.write(content)
