.PHONY: build run bench

build: 
	go build -o isuumo *.go

run:
	make build
	MYSQL_PORT=13306 ENV=dev ./isuumo

bench:
	sudo docker run --rm --network=host mayocream/isucon:10-quality-bench ./bench --target-url http://127.0.0.1:1323