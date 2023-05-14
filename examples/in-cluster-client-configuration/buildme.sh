GOOS=linux go build -o ./app .
docker build -t registry.central.aiware.com/in-cluster:qd .
docker push registry.central.aiware.com/in-cluster:qd

