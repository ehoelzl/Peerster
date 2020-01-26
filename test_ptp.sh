cd client/
go build
cd ..

go build

sleep 1
./Peerster -name=A -UIPort=8000 -gossipAddr=127.0.0.1:5000 -peers=127.0.0.1:5001,127.0.0.1:5002 -rtimer=20 -N=3 -hw3ex3=true > A.txt &
sleep 1
./Peerster -name=B -UIPort=8001 -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5002,127.0.0.1:5000 -rtimer=20 -N=3 -hw3ex3=true > B.txt &
sleep 1
./Peerster -name=C -UIPort=8002 -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5000,127.0.0.1:5001 -rtimer=20 -N=3 -hw3ex3=true > C.txt &
sleep 1



sleep 15

pkill -f Peerster
