GOOS=darwin     GOARCH=amd64    go build -ldflags '-s' -o bin/speedtunnel-darwin-amd64       main.go
GOOS=darwin     GOARCH=arm64    go build -ldflags '-s' -o bin/speedtunnel-darwin-arm64       main.go
GOOS=linux      GOARCH=386      go build -ldflags '-s' -o bin/speedtunnel-linux-386          main.go
GOOS=linux      GOARCH=amd64    go build -ldflags '-s' -o bin/speedtunnel-linux-amd64        main.go
GOOS=linux      GOARCH=arm      go build -ldflags '-s' -o bin/speedtunnel-linux-arm          main.go
GOOS=linux      GOARCH=arm64    go build -ldflags '-s' -o bin/speedtunnel-linux-arm64        main.go
GOOS=windows    GOARCH=386      go build -ldflags '-s' -o bin/speedtunnel-windows-386.exe    main.go
GOOS=windows    GOARCH=amd64    go build -ldflags '-s' -o bin/speedtunnel-windows-amd64.exe  main.go
