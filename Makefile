.PHONY: docker-up build run

docker-up:
	sudo systemctl start docker

build:
	docker build --network=host -t my-linter .

run:
	docker run --network=host my-linter:latest