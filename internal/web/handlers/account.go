package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/png"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/xdung24/conductor/internal/mailer"
)

// TwoFAPage renders the 2FA status page for the current user.
func (h *Handler) TwoFAPage(c *gin.Context) {
	username := h.username(c)
	_, enabled, _ := h.users.GetTOTP(username)
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "account_2fa.html", h.pageData(c, gin.H{
		"Enabled": enabled,
		"Flash":   flash,
		"Error":   "",
	}))
}

// TwoFASetupPage generates a new TOTP key, stores the pending secret, and
// renders the QR code so the user can scan it with their authenticator app.
func (h *Handler) TwoFASetupPage(c *gin.Context) {
	username := h.username(c)

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "ServiceMonitor",
		AccountName: username,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "account_2fa.html", gin.H{
			"Error": "Failed to generate 2FA key. Please try again.",
		})
		return
	}

	if err := h.users.SetTOTPSecret(username, key.Secret()); err != nil {
		c.HTML(http.StatusInternalServerError, "account_2fa.html", gin.H{
			"Error": "Failed to save 2FA key. Please try again.",
		})
		return
	}

	// Encode QR code image as a base64 data URI so it can be embedded directly
	// in the HTML without a separate image endpoint.
	qrImg, err := key.Image(200, 200)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "account_2fa.html", gin.H{
			"Error": "Failed to generate QR code.",
		})
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, qrImg); err != nil {
		c.HTML(http.StatusInternalServerError, "account_2fa.html", gin.H{
			"Error": "Failed to encode QR code.",
		})
		return
	}
	// #nosec G203 -- data URI is entirely server-generated base64 PNG; no user input is interpolated.
	qrDataURI := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))

	c.HTML(http.StatusOK, "account_2fa.html", h.pageData(c, gin.H{
		"Enabled":    false,
		"SetupMode":  true,
		"QRDataURI":  qrDataURI,
		"TOTPSecret": key.Secret(),
		"Error":      "",
		"Flash":      "",
	}))
}

// TwoFAVerify validates the TOTP code entered by the user after scanning the QR
// code and, on success, marks 2FA as enabled.
func (h *Handler) TwoFAVerify(c *gin.Context) {
	username := h.username(c)
	code := c.PostForm("code")

	secret, _, err := h.users.GetTOTP(username)
	if err != nil || secret == "" {
		c.HTML(http.StatusBadRequest, "account_2fa.html", gin.H{
			"Error": "No pending 2FA setup found. Please start the setup again.",
		})
		return
	}

	if !totp.Validate(code, secret) {
		// Re-render the setup page with the same QR code so the user can retry.
		keyURL := fmt.Sprintf("otpauth://totp/ServiceMonitor%%3A%s?secret=%s&issuer=ServiceMonitor",
			url.QueryEscape(username), secret)
		key, parseErr := otp.NewKeyFromURL(keyURL)

		var qrDataURI template.URL
		if parseErr == nil {
			if qrImg, imgErr := key.Image(200, 200); imgErr == nil {
				var buf bytes.Buffer
				if pngErr := png.Encode(&buf, qrImg); pngErr == nil {
					// #nosec G203 -- data URI is entirely server-generated base64 PNG; no user input is interpolated.
					qrDataURI = template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))
				}
			}
		}

		c.HTML(http.StatusUnauthorized, "account_2fa.html", gin.H{
			"Enabled":    false,
			"SetupMode":  true,
			"QRDataURI":  qrDataURI,
			"TOTPSecret": secret,
			"Error":      "Invalid code. Please try again.",
			"Flash":      "",
		})
		return
	}

	if err := h.users.EnableTOTP(username); err != nil {
		c.HTML(http.StatusInternalServerError, "account_2fa.html", gin.H{
			"Error": "Failed to enable 2FA. Please try again.",
		})
		return
	}

	// Notify the user that 2FA was enabled (fire-and-forget).
	h.mailer.SendAsync(username, "Two-factor authentication enabled", mailer.RenderTwoFAEnabled())

	c.SetCookie("sm_flash", "Two-factor authentication has been enabled.", 60, "/", "", false, true)
	c.Redirect(http.StatusFound, "/account/2fa")
}

// TwoFADisable disables TOTP for the current user and clears the stored secret.
func (h *Handler) TwoFADisable(c *gin.Context) {
	username := h.username(c)
	if err := h.users.DisableTOTP(username); err != nil {
		c.HTML(http.StatusInternalServerError, "account_2fa.html", gin.H{
			"Error": "Failed to disable 2FA. Please try again.",
		})
		return
	}
	c.SetCookie("sm_flash", "Two-factor authentication has been disabled.", 60, "/", "", false, true)
	c.Redirect(http.StatusFound, "/account/2fa")
}
