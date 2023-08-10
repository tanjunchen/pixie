#!/bin/bash

set -e

src_repo=gcr.io
source_user=pixie-oss

dest_repo=registry.baidubce.com
dest_user=csm

while read sc_image; do
    if [ -z "${sc_image}" ]
    then
      continue
    fi
    echo "docker pull --platform=linux/amd64 ${sc_image}"
    docker pull --platform=linux/amd64 ${src_repo}/${source_user}/${sc_image}
    docker tag ${src_repo}/${source_user}/${sc_image} ${dest_repo}/${dest_user}/${sc_image}
    docker push  ${dest_repo}/${dest_user}/${sc_image}
done < images
