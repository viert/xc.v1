MAIN = src/xc.go
DEPS = github.com/viert/properties \
		github.com/viert/sekwence \
		github.com/viert/smartpty \
		github.com/chzyer/readline \
		github.com/kr/pty \
		github.com/npat-efault/poller \
		github.com/svent/go-nbreader \
		gopkg.in/cheggaaa/pb.v1 \
		github.com/op/go-logging \
		github.com/go-ini/ini

OSTYPE = $(shell uname -s)
ENV = GOPATH=$(CURDIR)
# ifeq ($(OSTYPE),Linux)
# 	ENV += CGO_ENABLED=0
# endif

SOURCE = src/xc.go src/cli/*.go src/config/*.go src/conductor/*.go src/executer/*.go \
			src/remote/*.go src/term/*.go

xc: $(SOURCE)
	env $(ENV) go build $(MAIN)

deps:
	env $(ENV) go get $(DEPS)

clean:
	rm xc
