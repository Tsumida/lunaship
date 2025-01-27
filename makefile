# env def
COVERAGE_DIR="./coverage"
COVERAGE_FILE=${COVERAGE_DIR}"/coverage.out"

# cmd
proto:
	@cd ./api && buf genenrate .

test: 
	@mkdir -p ${COVERAGE_DIR} && touch ${COVERAGE_FILE}
	@go test -count=1 -coverprofile=${COVERAGE_FILE} -v -timeout 60s ./infra/...
	@go tool cover -html=${COVERAGE_FILE} -o ./coverage/coverage.html

list-env:
	@./script/list_env.sh