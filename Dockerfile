FROM golang
RUN mkdir /src
ADD Makefile /src/Makefile
RUN cd /src && make deps
ADD src /src/src
ADD build.sh /src/build.sh
WORKDIR /src
ENTRYPOINT /src/build.sh
