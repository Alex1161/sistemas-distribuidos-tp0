# Ejecucion
## Prerequisitos
- Tener instalado **Docker**
- Tener instalado **Python3**
- Tener instalado **Make**
  
## Parte 1
### Ejercicio 1
Para ejecutar este ejercicio se agrego el parametro CLIENT al makefile y un script en python. Por lo cual se debera seguir los siguientes pasos.

1. Abrir una terminal
2. Moverse a la raiz del proyecto.
3. Moverse a la branch exercise_1 usando el comando de git `git checkout exercise_1`
4. Ejecutar el comando `make docker-compose-up CLIENTS=N`, siendo N la cantidad de clientes que se desea levantar (e.g. `make docker-compose-up CLIENTS=4`).
5. Ejecutar el comando `make docker-compose-logs`
6. Ejecutar el comando `make docker-compose-down`

### Ejercicio 3
Para ejecutar este ejercicio se agrego el parametro ENV al makefile. Por lo cual se debera seguir los siguientes pasos.

1. Abrir una terminal
2. Moverse a la raiz del proyecto.
3. Moverse a la branch exercise_3 usando el comando de git `git checkout exercise_3`
4. Ejecutar el comando `sudo make docker-compose-up ENV=TEST`.
5. Ejecutar el comando `sudo make docker-compose-logs ENV=TEST`
6. Ejecutar el comando `sudo make docker-compose-down ENV=TEST`

### Resto de ejercicios
Para el resto de ejercicios se deberar usar el mismo procedimiento:

1. Abrir una terminal
2. Moverse a la raiz del proyecto.
3. Moverse a la branch en cuestion usando el comando de git `git checkout exercise_<exercise>`
4. Ejecutar el comando `sudo make docker-compose-up`.
5. Ejecutar el comando `sudo make docker-compose-logs`
6. Ejecutar el comando `sudo make docker-compose-down`


## Parte 2
## Protocolo
Para este ejercicio se implemento el siguiente protocolo binario que se divide en 2 etapas:

### Etapa 1, agregado de datos
1. El cliente enviara un paquete en Big Endian que tendra la forma: 
   ```
    <SIZE_CONTENT>
    <AGENCY>;
    <name_1>;<lastname_1>;<document_1>;<birthday_1>;<number_1>;
    <name_2>;<lastname_2>;<document_2>;<birthday_2>;<number_2>;
    ......
    <name_n>;<lastname_n>;<document_n>;<birthday_n>;<number_n>
    <CONTINUE>
   ```
2. El servidor recibira dicho paquete y sabra lo siguiente:
   1. Cuantos bytes esperar por el campo `<SIZE_CONTENT>` que mide 2 bytes e indica el tamaño del resto del paquete
   2. Si esperar mas paquetes por el campo `<CONTINUE>` el cual mide tambien 2 bytes e indica si se seguiran enviando mas paquetes
   3. Y de que agencia viene por el campo `<AGENCY>`
   4. Como parsear la informacion ya que ademas que sabe que todo estara en Bigendian, siempre el contenido viene en el orden especificado, primero el **name**, luego el **lastname** y asi sucesivamente.
3. Una vez el servidor haya guardado las apuestas, envia un ACK que es un int equivalente a 1 en el cual le informa al cliente que proceso bien la informacion.
4. El cliente lo recibe y continua enviando los paquetes.

### Etapa 2, descubrimiento del ganador
1. Cuando el cliente esta por enviar el ultimo paquete se cambia el valor de `<CONTINUE>` a 0, una vez recibe el ACK ahora en vez de enviar paquetes le pide el ganador enviandole la agencia, uedando el paquete asi: `<SIZE><AGENCY>;<CONTINUE>`
2. Una vez todos los clientes hayan enviado su ultimo paquete, i.e. enviar un paquete con el valor de `<CONTINUE>` en 0, procede enviarles a los clientes sus respectivos ganadores con un paquete de la siguiente forma:   
    `<SIZE><winner_document_1>;<winner_document_2>;<winner_document_3>;...;<winner_document_n>`
Y cierra la comunicacion
3. El cliente lo recibe y tambien termina la comunicacion con el servidor

## Parte 3
Para hacer que el servidor maneje las peticiones de los 5 clientes concurrentemente se usaron procesos, cada vez que llegaba una nueva conecion se levantaba un nuevo proceso y este se encargaba de recibir, pasear, armar y enviar los paquetes con su respectivo cliente hasta que se enviara el ganador, una vez hecho esto se une con el hilo principal

Como detalle de implementacion, dado al protocolo implementado se tiene 2 etapas muy marcadas, el envio de apuestas y el descubrimiento del ganador, por lo que se puede ver reflejado en la implementacion de la concurrencia. 

```
def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        # TODO: Modify this program to handle signal to graceful shutdown
        # the server
        with Manager() as manager:
            connections = manager.list()
            database_lock = Lock()
            while not self._shutdown and len(self._process) < MAX_CONNECTIONS:
                conn = self.__accept_new_connection()
```
Aqui se levanta los procesos para la primera etapa
```
                p = Process(
                    target=self.__handle_client_connection,
                    args=(conn, connections, database_lock)
                )
                p.start()
                self._process.append(p)

            for p in self._process:
                p.join()
```
Y luego aqui se sincronizan para pasar todos a la siguiente etapa, basicamente actua como una barrera
```

            self._process = []
            logging.info(f'action: sorteo | result: success')
            database_lock.acquire()
            all_bets = load_bets()
            database_lock.release()
            winners = list(filter(lambda b: has_won(b), all_bets))

            while not self._shutdown and len(self._process) < MAX_CONNECTIONS:
                conn = connections.pop(0)
```
Otro detalle es que durante ambas etapas no se cierran las conexiones con los clientes, por lo que el cliente que termine rapidamente de enviar sus apuestas y quiera obtener rapidamente su ganador tendra que esperar a los demas.
```
                p = Process(
                    target=self.__handle_client_get_winners,
                    args=(conn, winners,)
                )
                p.start()
                self._process.append(p)

            for p in self._process:
                p.join()

        logging.info(f'action: server_done | result: success')
```

Por ultimo ademas del `join` usado para sincronizar las etapas, se usaron locks para sincronizar el acceso al guardado de las apuestas y el agregado de conexiones; para esta segunda se uso el Manager de tipo lista de la libreria standar multiprocessing de python el cual usa sincroniza los accesos a dicha variable teniendo como resultado un uso transparente de esta.
```
    def __handle_client_connection(self, connection, connections, database_lock):
        ......
        bytes_msg = self.__recv(connection)
                if connection.state == RECV_BETS:
                    connection.agency, bets = self.__decode(bytes_msg)
```
Aqui vemos como se bloquea el acceso al guardado de los datos para los demas procesos usando locks.
```
                    database_lock.acquire()
                    store_bets(bets)
                    database_lock.release()
                else:
                    connections.append(connection)
        ......
```