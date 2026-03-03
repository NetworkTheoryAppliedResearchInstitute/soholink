@echo off
title SoHoLINK Node
echo Starting SoHoLINK Node...
echo.
echo  HTTP API  : http://localhost:8080
echo  RADIUS    : 0.0.0.0:1812  (auth)
echo  RADIUS    : 0.0.0.0:1813  (accounting)
echo.
echo Press Ctrl+C to stop the node.
echo ----------------------------------------
fedaaa.exe start
echo.
echo Node stopped. Press any key to close.
pause >nul
