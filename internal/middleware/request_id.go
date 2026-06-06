package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

// RequestID returns an Echo middleware that ensures every request carries an
// X-Request-ID header. Precedence:
//  1. honour the inbound X-Request-ID if the client supplied one;
//  2. else use the active OTel trace ID so logs, traces, and the response
//     header all share the same correlation key;
//  3. else generate a fresh UUID.
//
// The chosen ID is set on both the response header and the request header so
// downstream middlewares (notably RequestLogger) can pick it up.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			rid := req.Header.Get(echo.HeaderXRequestID)
			if rid == "" {
				if span := trace.SpanFromContext(req.Context()); span.SpanContext().IsValid() {
					rid = span.SpanContext().TraceID().String()
				} else {
					rid = uuid.NewString()
				}
			}
			req.Header.Set(echo.HeaderXRequestID, rid)
			res.Header().Set(echo.HeaderXRequestID, rid)
			return next(c)
		}
	}
}
