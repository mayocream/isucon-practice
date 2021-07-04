clean:
	rm -rf ./lib ./webapp

lib: 
	apt update && apt install -y python-pip python3-pip
	git clone --depth 1 https://github.com/isucon/isucon10-qualify.git ./lib
	cd lib/initial-data && pip3 install -r requirements.txt && go get github.com/orisano/wayt && make chair_data && make estate_data
	cp webapp/fixture/* fixture/
	cp webapp/db/* mysql/migration/