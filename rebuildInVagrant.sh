docker run --name builder --rm -v /vagrant/build:/app digibib/build cp app /app

docker build -t digibib/docker-compose-dot -f /vagrant/Dockerfile /vagrant

docker save -o /vagrant/build/docker-compose-dot.dockerImage digibib/docker-compose-dot
