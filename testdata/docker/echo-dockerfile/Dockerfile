FROM golang:1.22-alpine3.19
ADD http-echo.go /
ADD go.mod /
RUN cd / && go build http-echo.go
CMD ["/http-echo"]

