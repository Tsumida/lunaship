# env def

# cmd
proto:
	@cd ./api && buf genenrate .

test: 
	go test -count=1 -v -timeout 60s ./...