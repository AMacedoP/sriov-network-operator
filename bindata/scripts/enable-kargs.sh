#!/bin/bash
set -x

REDHAT_RELEASE_FILE="/host/etc/redhat-release"

declare -a kargs=( "$@" )
ret=0
if grep --quiet CoreOS "$REDHAT_RELEASE_FILE"; then
    args=$(chroot /host/ rpm-ostree kargs)
    for t in "${kargs[@]}";do
        if [[ $args != *${t}* ]];then
            chroot /host/ rpm-ostree kargs --append ${t} > /dev/null 2>&1
            let ret++
        fi
    done
else
    chroot /host/ which grubby > /dev/null 2>&1
    # if grubby is not there, let's tell it
    if [ $? -ne 0 ]; then
        exit 127
    fi

    eval `chroot /host/ grubby --info=DEFAULT | grep args`
    for t in "${kargs[@]}";do
        if [[ $args != *${t}* ]];then
            chroot /host/ grubby --update-kernel=DEFAULT --args=${t} > /dev/null 2>&1
            let ret++
        fi
    done
fi
echo $ret
