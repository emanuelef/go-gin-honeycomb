#!/bin/bash

unset token
prompt="Honeycomb API Key: "
while IFS= read -p "$prompt" -r -s -n 1 char; do
    if [[ $char == $'\0' ]]; then
        break
    fi
    prompt='*'
    token+="$char"
done
echo

sed "s/your_key_here/$token/" .env.example >.env
sed "s/your_key_here/$token/" ./secondary/.env.example >./secondary/.env
sed "s/your_key_here/$token/" ./grpc-server/.env.example >./grpc-server/.env
