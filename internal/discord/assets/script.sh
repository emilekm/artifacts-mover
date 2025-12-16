#!/bin/bash

for file in *.png
do
    convert "$file" "$(basename "$file" .png).png"
done
