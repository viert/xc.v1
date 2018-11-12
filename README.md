## XC

xc is a fast parallel remote executer written in Go

#### How to build

```
export PROJECTDIR=xc
git clone git@gitlab.corp.mail.ru:mntdev/xc $PROJECTDIR
cd $PROJECTDIR
export GOPATH=$PWD
go get github.com/viert/properties
go get github.com/viert/smartpty 
go get github.com/chzyer/readline
go get github.com/svent/go-nbreader
go get github.com/viert/sekwence
go get gopkg.in/cheggaaa/pb.v1
go build src/xc.go
```
