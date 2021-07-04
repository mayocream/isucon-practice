clean:
	rm -rf ./lib ./webapp

lib: 
	apt update && apt install -y python-pip python3-pip
	git clone --depth 1 https://github.com/isucon/isucon10-qualify.git ./lib
	cd lib/initial-data && pip3 install -r requirements.txt && go get github.com/orisano/wayt && make chair_data && make estate_data
	cp webapp/fixture/* fixture/
	cp webapp/db/* mysql/migration/

nginxlog:
	goaccess log/nginx/access.log --log-format=COMBINED > log/nginx/report.html
	cd log/nginx/ && python3 -m http.server 7800

install-goaccess:
	wget https://tar.goaccess.io/goaccess-1.5.1.tar.gz
	tar -xzvf goaccess-1.5.1.tar.gz
	cd goaccess-1.5.1/ && ./configure --enable-utf8 && make && make install
	rm -rf goaccess-1.5.1/ goaccess-1.5.1.tar.gz