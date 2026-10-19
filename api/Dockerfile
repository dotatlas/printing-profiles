# Build stage: compile one static binary.
FROM golang:1.27 AS build
WORKDIR /src
# Copy the module files first, so Docker can keep the download layer when only the code changes.
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api .

# Run stage: only the binary, on a small base image with no shell, as a user that is not root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
CMD ["serve"]
