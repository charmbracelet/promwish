# promwish

Package promwish provides a simple [wish](http://github.com/charmbracelet/wish) middleware exposing some Prometheus metrics.

## Example Usage

You can add `promwish` as a middleware to your app:

```go
promwish.Middleware("localhost:9222", "my-app"),
```

This will create the metrics and start a HTTP server on `localhost:9222` to expose the metrics.

You can also use `promwish.MiddlewareRegistry` and `promwish.Listen` if you need more options.

Check the [_examples folder](/_examples) for a full working example.

## Example Dashboard

<img width="2120" alt="image" src="https://user-images.githubusercontent.com/245435/145126273-2dc9cb98-7886-40b5-b173-229c50746fba.png">
