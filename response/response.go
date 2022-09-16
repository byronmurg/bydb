package response

type Response struct {
	Code uint64
	Body string
}

func NotFound() *Response {
	return &Response{ Code:404, Body: "not found" }
}

func Conflict() *Response {
	return &Response{ Code:409, Body: "conflict" }
}

func Success(body string) *Response {
	return &Response{ Code:200, Body: body }
}
