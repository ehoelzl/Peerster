cd client/
go build
cd ..

go build

sleep 1
./Peerster -name=A -UIPort=8000 -gossipAddr=127.0.0.1:5000 -peers=127.0.0.1:5001 -rtimer=20 -N=3 > A.txt &
sleep 1

./Peerster -name=B -UIPort=8001 -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5002 -rtimer=20 -N=3 > B.txt &
sleep 1
./client/client -UIPort=8000 -file=duck.jpg

sleep 10
./Peerster -name=C -UIPort=8002 -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5003 -rtimer=20 -N=3 > C.txt &

sleep 10

pkill -f Peerster
