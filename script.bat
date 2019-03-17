@echo off
::echo %*
:::::::::::::::::::::::::::::::::::::::
echo %~n1
:: expands %1 to a file name only
echo %~x1
:: expands %1 to a file extension only
:::::::::::::::::::::::::::::::::::::::
::echo %~1
:: expands %1 removing any surrounding quotes (")

::echo %~f1
:: expands %1 to a fully qualified path name

::echo %~d1
:: expands %1 to a drive letter only

::echo %~p1
:: expands %1 to a path only

::echo %~s1
:: expanded path contains short names only

::echo %~a1
:: expands %1 to file attributes

::echo %~t1
:: expands %1 to date/time of file

::echo %~z1
:: expands %1 to size of file