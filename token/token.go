package token

import (
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ErizJ/JG0"
	"net/http"
	"time"
)

type JwtHandler struct {
	Alg            string
	Timeout        time.Duration
	RefreshTimeout time.Duration
	TimeFun        func() time.Time
	Key            []byte
	PrivateKey     string
	RefreshKey     string
	SendCookie     bool
	CookieName     string
	CookieMaxAge   int
	CookieDomain   string
	SecureCookie   bool
	CookieHTTPOnly bool
	Authenticator  func(ctx *msgo.Context) (map[string]any, error)
	Header         string
	AuthHandler    func(ctx *msgo.Context, err error)
}

type JwtResponse struct {
	Token        string
	RefreshToken string
}

const JWTToken = "jwt_token"

func (j *JwtHandler) LogoutHandler(ctx *msgo.Context) error {
	//清除cookie即可
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		ctx.SetCookie(j.CookieName, "", -1, "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
	}
	return nil
}

func (j *JwtHandler) LoginHandler(ctx *msgo.Context) (*JwtResponse, error) {
	data, err := j.Authenticator(ctx)
	if err != nil {
		return nil, err
	}
	if j.Alg == "" {
		j.Alg = "HS256"
	}
	if j.TimeFun == nil {
		j.TimeFun = func() time.Time {
			return time.Now()
		}
	}
	signingMethod := jwt.GetSigningMethod(j.Alg)
	token := jwt.New(signingMethod)
	claims := token.Claims.(jwt.MapClaims)
	if data != nil {
		for k, v := range data {
			claims[k] = v
		}
	}
	expire := j.TimeFun().Add(j.Timeout)
	//
	claims["exp"] = expire.Unix()
	claims["iat"] = j.TimeFun().Unix()
	var tokenStr string
	var tokenErr error
	if j.usingPublicKeyAlgo() {
		tokenStr, tokenErr = token.SignedString(j.PrivateKey)
	} else {
		tokenStr, tokenErr = token.SignedString(j.Key)
	}
	if tokenErr != nil {
		return nil, tokenErr
	}
	refreshTokenStr, tokenErr := j.refreshToken(token)
	if tokenErr != nil {
		return nil, tokenErr
	}
	req := &JwtResponse{
		Token:        tokenStr,
		RefreshToken: refreshTokenStr,
	}
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		if j.CookieMaxAge == 0 {
			j.CookieMaxAge = int(expire.Unix() - j.TimeFun().Unix())
		}
		maxAge := j.CookieMaxAge
		ctx.SetCookie(j.CookieName, tokenStr, maxAge, "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
	}
	return req, nil

}

func (j *JwtHandler) usingPublicKeyAlgo() bool {
	switch j.Alg {
	case "RS256", "RS512", "RS384":
		return true
	}
	return false
}

func (j *JwtHandler) refreshToken(token *jwt.Token) (string, error) {
	claims := token.Claims.(jwt.MapClaims)
	expire := j.TimeFun().Add(j.RefreshTimeout)
	claims["exp"] = expire.Unix()
	var tokenStr string
	var tokenErr error
	if j.usingPublicKeyAlgo() {
		tokenStr, tokenErr = token.SignedString(j.PrivateKey)
	} else {
		tokenStr, tokenErr = token.SignedString(j.Key)
	}
	if tokenErr != nil {
		return "", tokenErr
	}
	return tokenStr, nil
}

func (j *JwtHandler) RefreshTokenHandler(ctx *msgo.Context) (*JwtResponse, error) {
	rToken, ok := ctx.Get(j.RefreshKey)
	if !ok {
		return nil, errors.New("refresh token is null")
	}
	if j.Alg == "" {
		j.Alg = "HS256"
	}
	if j.TimeFun == nil {
		j.TimeFun = func() time.Time {
			return time.Now()
		}
	}
	//解析token
	token, err := jwt.Parse(rToken.(string), func(token *jwt.Token) (interface{}, error) {
		if j.usingPublicKeyAlgo() {
			return j.PrivateKey, nil
		} else {
			return j.Key, nil
		}
	})
	if err != nil {
		return nil, err
	}
	claims := token.Claims.(jwt.MapClaims)
	ctx.Logger.Info(claims)
	expire := j.TimeFun().Add(j.Timeout)
	//
	claims["exp"] = expire.Unix()
	claims["iat"] = j.TimeFun().Unix()
	var tokenStr string
	var tokenErr error
	if j.usingPublicKeyAlgo() {
		tokenStr, tokenErr = token.SignedString(j.PrivateKey)
	} else {
		tokenStr, tokenErr = token.SignedString(j.Key)
	}
	if tokenErr != nil {
		return nil, tokenErr
	}
	refreshTokenStr, tokenErr := j.refreshToken(token)
	if tokenErr != nil {
		return nil, tokenErr
	}
	req := &JwtResponse{
		Token:        tokenStr,
		RefreshToken: refreshTokenStr,
	}
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		if j.CookieMaxAge == 0 {
			j.CookieMaxAge = int(expire.Unix() - j.TimeFun().Unix())
		}
		maxAge := j.CookieMaxAge
		ctx.SetCookie(j.CookieName, tokenStr, maxAge, "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
	}
	return req, nil

}

func (j *JwtHandler) AuthInterceptor(next msgo.HandlerFunc) msgo.HandlerFunc {
	return func(ctx *msgo.Context) {
		if j.Header == "" {
			j.Header = "Authorization"
		}
		token := ctx.R.Header.Get(j.Header)
		if token == "" {
			if j.SendCookie {
				cookieToken, err := ctx.GetCookie(j.CookieName)
				if err != nil {
					if j.AuthHandler == nil {
						ctx.W.WriteHeader(http.StatusUnauthorized)
					} else {
						j.AuthHandler(ctx, err)
					}
					return
				}
				token = cookieToken
			} else {
				if j.AuthHandler == nil {
					ctx.W.WriteHeader(http.StatusUnauthorized)
				} else {
					j.AuthHandler(ctx, errors.New("token is empty"))
				}
				return
			}
		}
		//解析token
		t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			if j.usingPublicKeyAlgo() {
				return j.PrivateKey, nil
			} else {
				return j.Key, nil
			}
		})
		if err != nil {
			if j.AuthHandler == nil {
				ctx.W.WriteHeader(http.StatusUnauthorized)
			} else {
				j.AuthHandler(ctx, err)
			}
			return
		}
		ctx.Set("claims", t.Claims.(jwt.MapClaims))
		next(ctx)
	}
}
