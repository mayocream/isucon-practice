clean:
	rm -rf ./lib ./webapp

old-lib: 
	apt update && apt install -y python-pip python3-pip
	git clone --depth 1 https://github.com/isucon/isucon10-qualify.git ./lib
	cd lib/initial-data && pip3 install -r requirements.txt && go get github.com/orisano/wayt && make chair_data && make estate_data
	cp webapp/fixture/* fixture/
	cp webapp/db/* mysql/migration/

lib:
	chmod +x lib.sh && ./lib.sh

nginxlog:
	goaccess log/nginx/access.log --log-format=COMBINED > log/nginx/report.html
	cd log/nginx/ && python3 -m http.server 7800

install-goaccess:
	wget https://tar.goaccess.io/goaccess-1.5.1.tar.gz
	tar -xzvf goaccess-1.5.1.tar.gz
	cd goaccess-1.5.1/ && ./configure --enable-utf8 && make && make install
	rm -rf goaccess-1.5.1/ goaccess-1.5.1.tar.gz

bench:
	docker run --rm --network=host mayocream/isucon:10-quality-bench ./bench --target-url http://127.0.0.1:1234

build-bench:
	sudo docker build -t isucon10-qualify-bench:dev -f Dockerfile-benchmark .