.PHONY: all issumo run

all: isuumo

isuumo: *.go
	go build -o isuumo *.go

run:
	MYSQL_PORT=13306 isuumo

bench:
	sudo docker run --rm --network=host mayocream/isucon:10-quality-bench ./bench --target-url http://127.0.0.1:1323