#!/bin/bash
set -xe
set -o pipefail

MYSQL_HOST=${MYSQL_HOST:-127.0.0.1}
MYSQL_PORT=${MYSQL_PORT:-13306}
MYSQL_USER=${MYSQL_USER:-isucon}
MYSQL_DBNAME=${MYSQL_DBNAME:-isuumo}
MYSQL_PWD=${MYSQL_PASS:-isucon}
LANG="C.UTF-8"

mysql -h $MYSQL_HOST -P $MYSQL_PORT -u isucon -pisucon isuumo < app/0_Schema.sql
mysql -h $MYSQL_HOST -P $MYSQL_PORT -u isucon -pisucon isuumo < app/0_Index.sql
mysql -h $MYSQL_HOST -P $MYSQL_PORT -u isucon -pisucon isuumo < mysql/db/1_DummyEstateData.sql
mysql -h $MYSQL_HOST -P $MYSQL_PORT -u isucon -pisucon isuumo < mysql/db/2_DummyChairData.sql
