# Galera Images

The idea is to have all database and cluster logics managed by the operator. So database images need to be standard images. The choice was to use MariaDB but there is no special features except mariabackup that could not run on mysql or percona images.

Unfortunately, it was impossible to use [MariaDB images](https://hub.docker.com/_/mariadb) from Docker Hub because the image is build for stand alone server and in case of clustering, a reboot is needed. I was impossible to use as it for the Galera Operator. So only a modification is done in the last script to add the fact that the database is initialized.

Galera Images Dockerfile will look like that :

```bash
# vim:set ft=dockerfile:
FROM mariadb:10.4.12-bionic

COPY docker-entrypoint.sh /usr/local/bin/
ENTRYPOINT ["docker-entrypoint.sh"]

EXPOSE 3306
CMD ["mysqld"]
```

With the docker-entrypoint.sh having only a minor modification in the beginning of the script (line 290):

```bash
...
_main() {
	# if command starts with an option, prepend mysqld
	if [ "${1:0:1}" = '-' ]; then
		set -- mysqld "$@"
	fi

	# skip setup if they aren't running mysqld or want an option that stops mysqld
	if [ "$1" = 'mysqld' -a -n "$CLUSTER_INIT" ] && ! _mysql_want_help "$@"; then
		mysql_note "Entrypoint script for MySQL Server ${MARIADB_VERSION} started."

		mysql_check_config "$@"
		# Load various environment variables
		docker_setup_env "$@"
		docker_create_db_directories
...
```

instead of 

```bash
...
_main() {
	# if command starts with an option, prepend mysqld
	if [ "${1:0:1}" = '-' ]; then
		set -- mysqld "$@"
	fi

	# skip setup if they aren't running mysqld or want an option that stops mysqld
	if [ "$1" = 'mysqld' ] && ! _mysql_want_help "$@"; then
		mysql_note "Entrypoint script for MySQL Server ${MARIADB_VERSION} started."

		mysql_check_config "$@"
		# Load various environment variables
		docker_setup_env "$@"
		docker_create_db_directories
...
```

A new env variable is used (CLUSTER_INIT). The prupose is to let the first node being initialized. Other nodes will use a galera replication at the startup sequence.

Do not forget to use the same rights for the docker-entrypoint.sh script : `chmod 775 docker-entrypoint.sh`


