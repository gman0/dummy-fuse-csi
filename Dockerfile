FROM alpine:3.18.3

RUN apk add --no-cache libc6-compat fuse3

COPY dummy-fuse /bin/dummy-fuse
COPY dummy-fuse-csi /bin/dummy-fuse-csi
COPY dummy-fuse-workload /bin/dummy-fuse-workload
