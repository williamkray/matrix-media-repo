package common

const ErrCodeInvalidHost = "M_INVALID_HOST"
const ErrCodeNotFound = "M_NOT_FOUND"
const ErrCodeUnknownToken = "M_UNKNOWN_TOKEN"
const ErrCodeNoGuests = "M_GUEST_ACCESS_FORBIDDEN"
const ErrCodeMissingToken = "M_MISSING_TOKEN"
const ErrCodeMediaTooLarge = "M_MEDIA_TOO_LARGE"
const ErrCodeMediaTooSmall = "M_MEDIA_TOO_SMALL"
const ErrCodeTooLarge = "M_TOO_LARGE"
const ErrCodeMethodNotAllowed = "M_METHOD_NOT_ALLOWED"
const ErrCodeBadRequest = "M_BAD_REQUEST"
const ErrCodeRateLimitExceeded = "M_LIMIT_EXCEEDED"
const ErrCodeUnknown = "M_UNKNOWN"
const ErrCodeForbidden = "M_FORBIDDEN"
const ErrCodeUnauthorized = "M_UNAUTHORIZED"
const ErrCodeQuotaExceeded = "M_QUOTA_EXCEEDED"
const ErrCodeCannotOverwrite = "M_CANNOT_OVERWRITE_MEDIA"
const ErrCodeNotYetUploaded = "M_NOT_YET_UPLOADED"
const ErrCodeUnavailable = "M_UNAVAILABLE"          // internal code only; used to pick an HTTP 503
const ErrCodeRemoteTooLarge = "M_REMOTE_TOO_LARGE"  // internal code only; used to pick an HTTP 502
const ErrCodeCannotThumbnail = "M_CANNOT_THUMBNAIL" // internal code only; used to pick an HTTP 400
