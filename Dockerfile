FROM golang
RUN mkdir /src
ADD src /src/src
ADD Makefile /src/Makefile
ADD build.sh /src/build.sh
RUN cd /src && make deps
WORKDIR /src
ENTRYPOINT /src/build.sh
