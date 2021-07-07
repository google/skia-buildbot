files=$(find . -name "probersk.json5")
echo $files
for i in $files
do
    mv "$i" "${i/5/}"
done