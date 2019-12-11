test:
	go test -v -failfast -timeout 5s -race ./...

cover:
	go test -coverprofile=go-cover.profile -timeout 5s ./...
	go tool cover -html=go-cover.profile
	rm go-cover.profile
