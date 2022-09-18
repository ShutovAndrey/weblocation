FROM golang:1.18-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

RUN go build  -o /weblocation

FROM gcr.io/distroless/base:latest

WORKDIR /

COPY --from=build /weblocation /weblocation

EXPOSE 8080

# USER nonroot:nonroot

ENTRYPOINT ["/weblocation"]