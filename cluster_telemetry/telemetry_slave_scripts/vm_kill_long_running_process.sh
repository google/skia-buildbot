ps -eo uid,pid,lstart,comm | tail -n+2 | grep timeout |
     while read PROC_UID PROC_PID _ _ _ PROC_LSTART PROC_CMD; do
         SECONDS=$[$(date +%s) - $(date -d"$PROC_LSTART" +%s)]
         if [ ${SECONDS#-} -gt 2700 ]; then
             echo $PROC_PID
         fi
      done |
      xargs sudo kill -9
