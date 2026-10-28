import { useState, useEffect } from "react";

interface CookieConsentProps {
  message: string;
  acceptLabel: string;
  declineLabel: string;
}

export function CookieConsent({
  message,
  acceptLabel,
  declineLabel,
}: CookieConsentProps) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const consent = localStorage.getItem("cookie-consent");
    if (consent === null) {
      setVisible(true);
    }
  }, []);

  const handleAccept = () => {
    localStorage.setItem("cookie-consent", "accepted");
    // Dispatch custom event so GA loader on the same page picks up the consent
    window.dispatchEvent(
      new StorageEvent("storage", {
        key: "cookie-consent",
        newValue: "accepted",
      }),
    );
    setVisible(false);
  };

  const handleDecline = () => {
    localStorage.setItem("cookie-consent", "declined");
    setVisible(false);
  };

  if (!visible) return null;

  return (
    <div
      style={{
        position: "fixed",
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: 9999,
        background: "rgba(10, 10, 10, 0.95)",
        borderTop: "1px solid rgba(255, 255, 255, 0.1)",
        backdropFilter: "blur(12px)",
      }}
    >
      <div
        style={{
          maxWidth: "1280px",
          margin: "0 auto",
          padding: "16px 24px",
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          justifyContent: "space-between",
          gap: "16px",
        }}
      >
        <p
          style={{
            color: "rgba(255, 255, 255, 0.6)",
            fontSize: "14px",
            margin: 0,
            flex: "1 1 300px",
          }}
        >
          {message}{" "}
          <a
            href={
              window.location.pathname.includes("/pt/")
                ? "/pt/privacy"
                : "/en/privacy"
            }
            style={{
              color: "rgba(255, 255, 255, 0.8)",
              textDecoration: "underline",
              textUnderlineOffset: "2px",
            }}
          >
            {window.location.pathname.includes("/pt/")
              ? "Politica de Privacidade"
              : "Privacy Policy"}
          </a>
        </p>
        <div style={{ display: "flex", gap: "12px", flexShrink: 0 }}>
          <button
            onClick={handleDecline}
            style={{
              padding: "8px 20px",
              fontSize: "14px",
              fontWeight: 500,
              color: "rgba(255, 255, 255, 0.5)",
              background: "transparent",
              border: "1px solid rgba(255, 255, 255, 0.15)",
              borderRadius: "8px",
              cursor: "pointer",
              transition: "all 0.2s",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = "rgba(255, 255, 255, 0.3)";
              e.currentTarget.style.color = "rgba(255, 255, 255, 0.8)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = "rgba(255, 255, 255, 0.15)";
              e.currentTarget.style.color = "rgba(255, 255, 255, 0.5)";
            }}
          >
            {declineLabel}
          </button>
          <button
            onClick={handleAccept}
            style={{
              padding: "8px 20px",
              fontSize: "14px",
              fontWeight: 500,
              color: "#000",
              background: "#fff",
              border: "none",
              borderRadius: "8px",
              cursor: "pointer",
              transition: "all 0.2s",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = "rgba(255, 255, 255, 0.85)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = "#fff";
            }}
          >
            {acceptLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
