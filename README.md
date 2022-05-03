# ProxyServer

Http прокси сервер и простой сканер уязвимости на его основе.

## Сборка и запуск

```shell
docker build -t proxy .
docker run -d -p 8080:8080 -p 8000:8000 -t proxy
```
## Пример использования

Запрос с помощью программы curl. Задаем адрес прокси сервера в опции -x:
```shell
curl -x http://127.0.0.1:8080 http://mail.ru
```

## Web api

На 8000 порту
```shell
/requests/{request_id:[0-9]+} – вывод 1 запроса
/requests – список запросов
/repeat/{request_id:[0-9]+} – повторная отправка запроса
/scan/{request_id:[0-9]+} – сканирование запроса
```
