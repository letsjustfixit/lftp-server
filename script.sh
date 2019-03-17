#!/bin/sh
echo "Script running.."
filename=$(basename -- "$@")
extension="${filename##*.}"
shortfilename="${filename%.*}"
echo $@
echo $?
 echo "Filename w/o ext: $filename\next:$extension"
if [ "$extension" = "wav" ]
then
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
   echo "> $FELTAMHU_URL/predikacio/$shortfilename"
   echo "{\"nas_basename\":\"$filename\",\"nas_filename\":\"$shortfilename\",\"web_basename\":\"$shortfilename.mp3\", \"web_size\":\"$FILESIZE\",\"action\":\"mp3convert\"}"
   curl -d "{\"nas_basename\":\"$filename\",\"nas_filename\":\"$shortfilename\",\"web_basename\":\"$shortfilename.mp3\", \"web_size\":\"$FILESIZE\",\"action\":\"mp3convert\"}" -H "Content-Type: application/json" -X POST "$FELTAMHU_URL/predikacio/$shortfilename"
  else
   echo "Not OK :'("
  fi # End of internal if
 fi #End of outer if
elif [ "$extension" = "mp3" ]
then
 echo "Mp3 file feltöltés.."
 if [ $? -eq 0 ]
  then
  FILESIZE=$(stat -c%s "$filename")
  echo "{\"nas_filename\":\"$shortfilename\",\"web_basename\":\"$shortfilename.mp3\", \"web_size\":\"$FILESIZE\",\"action\":\"upload_finished\"}"
  curl -d "{\"nas_filename\":\"$shortfilename\",\"web_basename\":\"$shortfilename.mp3\", \"web_size\":\"$FILESIZE\",\"action\":\"upload_finished\"}" -H "Content-Type: application/json" -X POST "$FELTAMHU_URL/predikacio/$shortfilename"
  else
  echo "Upload probably failed for file :( $filename"
 fi
else
 echo ">+ Se nem wav, se mp3, mit keres ez itt? :) +<";
fi
echo $?

