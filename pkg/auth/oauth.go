package auth

// OAuthSignatureHeaders is the list of headers that oauth-proxy includes in the HMAC signature.
var OAuthSignatureHeaders = []string{
	"Content-Length",
	"Content-Md5",
	"Content-Type",
	"Date",
	"Authorization",
	"X-Forwarded-User",
	"X-Forwarded-Email",
	"X-Forwarded-Access-Token",
	"Cookie",
	"Gap-Auth",
}

// GAPSignatureHeader is the name of the header containing the HMAC signature.
const GAPSignatureHeader = "GAP-Signature"
