cd client/
go build
cd ..

go build

sleep 1
./Peerster -name=A -UIPort=8000 -gossipAddr=127.0.0.1:5000 -peers=127.0.0.1:5001 -rtimer=20 -N=3 -hw3ex4=true -hopLimit=256 > A.txt &
sleep 1

./Peerster -name=B -UIPort=8001 -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5002 -rtimer=20 -N=3 -hw3ex4=true -hopLimit=256 > B.txt &
sleep 1
./client/client -UIPort=8000 -file=duck.jpg
sleep 1
./client/client -UIPort=8001 -file=duck_B.jpg
sleep 1
./client/client -UIPort=8000 -file=test.jpg
sleep 1
./client/client -UIPort=8001 -file=test_B.jpg

sleep 10
./Peerster -name=C -UIPort=8002 -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5001 -rtimer=20 -N=3 -hw3ex4=true -hopLimit=256 > C.txt &
sleep 1
./client/client -UIPort=8002 -file=duck_C.jpg
sleep 1
./client/client -UIPort=8002 -file=test_C.jpg

sleep 30

pkill -f Peerster
