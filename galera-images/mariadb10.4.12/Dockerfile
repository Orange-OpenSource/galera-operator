# vim:set ft=dockerfile:
FROM mariadb:10.4.12-bionic

COPY docker-entrypoint.sh /usr/local/bin/
ENTRYPOINT ["docker-entrypoint.sh"]

EXPOSE 3306
CMD ["mysqld"]