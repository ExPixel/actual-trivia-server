package auth

type loginResponse struct {
	AuthToken             string `json:"authToken"`
	AuthTokenExpiresAt    int64  `json:"authTokenExpiresAt"`
	RefreshToken          string `json:"refreshToken"`
	RefreshTokenExpiresAt int64  `json:"refreshTokenExpiresAt"`
}

type signupResponse struct {
	UserID int64 `json:"userID"`
}
