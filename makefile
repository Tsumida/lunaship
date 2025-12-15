# ==============================================================================
# 💾 本地开发
# ==============================================================================
list-env:
	@find ./ -type f -name "*.go" \
	-exec grep -oP 'os.Getenv\(\K"[A-Z_]+"(?=\))' {} \; \
	| sort \
	| uniq \
	| awk '{gsub(/"/, "", $$0); print "ENV", $$0 "=\"\""}'

list-redis-key:
	@find ./ -type f -name "*.go" \
	-exec grep -oP 'REDIS_KEY_[A-Z0-9_]+\s*=\s*"[a-z_\-]+"' {} \; \
	| awk -F'"' '{gsub(/REDIS_KEY_/, "", $$1); print $$2}'

pb:
	@buf generate -v --path ./api

test: 
	go test -count=1 -v -timeout 60s ./...