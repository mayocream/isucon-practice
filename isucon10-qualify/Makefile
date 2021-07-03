prepare: 
	wget https://github.com/mayocream/isucon-static/raw/master/isucon10-qualify/bench.tar.gz -O bench.tar.gz
	wget https://github.com/mayocream/isucon-static/raw/master/isucon10-qualify/webapp.tar.gz -O webapp.tar.gz
	tar zxvf bench.tar.gz
	tar zxvf webapp.tar.gz
	cp webapp/mysql/db/* mysql/migration/
	cp webapp/fixture/* fixture/
	rm -rf bench.tar.gz webapp.tar.gz