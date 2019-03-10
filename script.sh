#!/bin/sh
echo "Script running.."
filename=$(basename -- "$@")
extension="${filename##*.}"
shortfilename="${filename%.*}"
echo $@
#echo $?
# echo "Filename w/o ext: $filename\next:$extension"
if [ -e "$shortfilename.mp3" ]
then
 echo "> $shortfilename.mp3 already exists, no conversion required..."
else
 echo "> Converting to mp3..."
 lame  -b 128 --vbr-new -V 3 $filename "$shortfilename.mp3"
 if [ $? -eq 0 ]
 then
  echo "OK: [ $filename ] >> [ $shortfilename.mp3 ]"
  FILESIZE=$(stat -c%s "$shortfilename.mp3")
  echo "{\"nas_basename\":\"$filename\",\"nas_filename\":\"$shortfilename\",\"web_filename\":\"$shortfilename.mp3\", \"web_size\":\"$FILESIZE\"}"
  echo "> $FELTAMHU_URL/predikacio/$shortfilename"
  curl -d "{\"nas_basename\":\"$filename\",\"nas_filename\":\"$shortfilename\",\"web_filename\":\"$shortfilename.mp3\", \"web_size\":\"$FILESIZE\"}" -H "Content-Type: application/json" -X POST "$FELTAMHU_URL/predikacio/$shortfilename"
 else
  echo "Not OK :'("
 fi # End of internal if
fi #End of outer if

echo $?

