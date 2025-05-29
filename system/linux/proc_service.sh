APP_NAME=marude
FOLDER_NAME=/usr/local/marude
DESC="marude"

serv_install() {
	echo instaling...
	mkdir -p $FOLDER_NAME 2>/dev/null
	mkdir -p /etc/marude 2>/dev/null
	cp $1 $FOLDER_NAME
	cp marude_ctrl $FOLDER_NAME
	cp -rf view $FOLDER_NAME
	cp *.service $FOLDER_NAME
	cp conf/*.sample /etc/marude
	cp *.service /lib/systemd/system/
	systemctl daemon-reload
	systemctl enable $1
	systemctl start $1
}

serv_update() {
	systemctl stop $1
	cp $1 $FOLDER_NAME
	systemctl start $1
}

serv_start() {
	systemctl start $1
}

serv_stop() {
	systemctl stop $1
}

serv_uninstall() {
	systemctl stop $1
	systemctl disable $1
	rm /lib/systemd/system/marude*.service
	rm -rf $FOLDER_NAME
	rm -rf /etc/marude
}

serv_restart() {
	systemctl stop $1
	systemctl start $1
}

serv_status() {
	systemctl status $1
}

serv_help() {
    echo usage: /usr/local/marude/proc_service.sh [command] [process]
    echo
    echo   install     - install process as a service
    echo   update      - update service
    echo   start       - start service
    echo   stop        - stop service
    echo   restart     - restart servicd
    echo   uninstall   - uninstall service
    echo   status      - check the service status
}

if [ "$2" = "client" ]; then
	APP_NAME=marude_client
elif [ "$2" = "server" ]; then
	APP_NAME=marude_server
elif [ "$2" = "extend" ]; then
	APP_NAME=marude_extend
else
	serv_help
	exit 0
fi

if [ "$1" = "install" ]; then
	serv_install $APP_NAME
elif [ "$1" = "update" ]; then
	serv_update $APP_NAME
elif [ "$1" = "start" ]; then
	serv_start $APP_NAME
elif [ "$1" = "stop" ]; then
	serv_stop $APP_NAME
elif [ "$1" = "restart" ]; then
	serv_restart $APP_NAME
elif [ "$1" = "uninstall" ]; then
	serv_uninstall $APP_NAME
elif [ "$1" = "status" ]; then
	serv_status $APP_NAME
else
	serv_help
	exit 0
fi
