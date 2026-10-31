FROM ubuntu:latest
LABEL authors="vladsakharov"

ENTRYPOINT ["top", "-b"]