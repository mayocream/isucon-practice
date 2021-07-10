#!/bin/bash

id=$(sudo docker create mayocream/isucon:10-quality-bench)
sudo docker cp $id:/data/webapp/fixture/chair_condition.json fixture/
sudo docker cp $id:/data/webapp/fixture/estate_condition.json fixture/
sudo docker cp $id:/data/initial-data/result/1_DummyEstateData.sql mysql/db/
sudo docker cp $id:/data/initial-data/result/2_DummyChairData.sql mysql/db/
sudo docker rm -v $id