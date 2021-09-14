
ENV_LOCAL_TEST=\
  POSTGRES_PASSWORD=mysecretpassword \
  POSTGRES_DB=myawesomeproject \
  POSTGRES_HOST=postgres \
  POSTGRES_USER=postgres

# this command will start docker components that we set in docker-compose.yml
docker.setup:
  docker-compose up -d --remove-orphans;

# shutting down docker components
docker.stop:
  docker-compose down;

# this command will run all tests in the repo
# INTEGRATION_TEST_SUITE_PATH is used to run specific tests in Golang,
# if it's not specified it will run all tests

tests:
  $(ENV_LOCAL_TEST) \
  go test ./... -count=1 -v -run=$(INTEGRATION_TEST_SUITE_PATH)
