ps -eo uid,pid,lstart,comm | tail -n+2 | grep timeout | grep run_measurement |
     while read PROC_UID PROC_PID PROC_LSTART PROC_CMD; do
         SECONDS=$[$(date +%s) - $(date -d"$PROC_LSTART" +%s)]
         if [ $PROC_UID -eq 1000 -a $SECONDS -gt 2700 ]; then
             echo $PROC_PID
         fi
      done |
      xargs sudo kill
