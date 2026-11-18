# go-emailvalidator

[![CI](https://github.com/wenlingang/go-emailvalidator/actions/workflows/ci.yml/badge.svg)](https://github.com/wenlingang/go-emailvalidator/actions/workflows/ci.yml)

Email address validation for Go that answers the question that matters in
production: **can this address actually be sent to?**

*RFC-valid does not mean deliverable.* Most validators only check whether an
address is allowed by the RFCs. This library checks whether it can go out
through an ordinary SMTP channel, and tells you why not when it can't.

## The problem

A common failure chain:

1. A user types a note together with the address, e.g.
   `请改为此邮箱zhangsan@example.com`, into an email field.
2. The service validates it with `net/mail.ParseAddress`. That implements
   RFC 6532, which allows non-ASCII local parts, so validation passes.
3. The SMTP client RFC 2047-encodes the non-ASCII header value, producing an
   `=?UTF-8?q?...?=` encoded word instead of an address.
4. Re-parsing the envelope address now fails (`mail: no angle-addr`), and
   retrying never helps.

`ParseAddress("张三 <zhangsan@example.com>")` also succeeds, which is a
related trap. Popular regex-based validators have the same blind spot: some
permit the whole `\x{00A0}-\x{D7FF}` range in the local part.

## Install

```sh
go get github.com/wenlingang/go-emailvalidator
```

Requires Go 1.26 or newer (the minimum required by the current
`golang.org/x/net` and `golang.org/x/text`).

## Quick start

```go
package main

import (
	"errors"
	"fmt"

	"github.com/wenlingang/go-emailvalidator"
)

func main() {
	// Default configuration: ASCII profile, nothing extra allowed.
	fmt.Println(emailvalidator.IsValid("zhangsan@example.com")) // true

	err := emailvalidator.Validate("请改为此邮箱zhangsan@example.com")
	fmt.Println(errors.Is(err, emailvalidator.ErrNonASCIILocal)) // true
	fmt.Println(err)
	// emailvalidator: non-ASCII local part requires EAI profile at byte 0

	// Reuse a configured Validator; it is stateless and safe for concurrent use.
	v := emailvalidator.New(emailvalidator.WithProfile(emailvalidator.IDN))
	addr, err := v.Normalize("zhangsan@例子.中国")
	fmt.Println(addr, err) // zhangsan@xn--fsqu00a.xn--fiqs8s <nil>
}
```

Validation is a pure function: no network calls, no MX lookups.

## Profiles

| Profile | Local part | Domain | Use when |
|---|---|---|---|
| `ASCII` (default) | ASCII atext only | ASCII LDH only | Almost every ESP and SMTP relay |
| `IDN` | ASCII atext only | Unicode allowed; converted to punycode via IDNA2008 (`idna.Lookup`) and then checked | Your channel lacks SMTPUTF8, but the domain can be sent as punycode |
| `EAI` | ASCII atext plus Unicode (must be NFC: `Validate` returns `ErrNotNFC` for non-NFC input, while `Normalize` composes it to NFC first; no controls, spaces, format or private-use characters, mixed-script check) | Same as `IDN` | You have confirmed that the **entire** relay chain supports SMTPUTF8 |

> ### IDN and EAI: send the `Normalize` output, not the raw input
>
> With the `IDN` and `EAI` profiles, `Validate` returning `nil` means "valid
> **after conversion to punycode**". It does **not** mean the original string
> can be sent as is. Handing a raw Unicode domain to an SMTP library gets it
> RFC 2047-encoded, which fails exactly like the scenario above.
>
> `Normalize` trims whitespace, lowercases the domain, converts it to punycode
> and, under `EAI`, NFC-composes the local part (the local part's case is never
> changed).
>
> Always send the result of `Normalize` (or `Local + "@" + ASCIIDomain` from
> `Parse`):
>
> ```go
> v := emailvalidator.New(emailvalidator.WithProfile(emailvalidator.IDN))
> addr, err := v.Normalize(userInput) // trims, lowercases the domain, punycodes it
> if err != nil {
> 	return err
> }
> send(addr) // not userInput
> ```

## Options

| Option | Effect |
|---|---|
| `WithProfile(p)` | Select `ASCII`, `IDN` or `EAI` |
| `AllowQuotedLocal()` | Accept `"john doe"@example.com` |
| `AllowIPLiteral()` | Accept `a@[192.0.2.1]` and `a@[IPv6:2001:db8::1]` |
| `AllowNoTLD()` | Accept single-label domains such as `a@localhost` |
| `AllowMixedScript()` | Turn off the mixed-script check (`EAI` only) |

By default the library also rejects things RFC 5322 tolerates but which are
nearly always dirty data: display names (`Name <a@example.com>`), comments,
quoted local parts, IP literals and domains without a TLD.

Notes on edge behavior: the mixed-script check is a simplified UTS #39
"Highly Restrictive" subset (scripts not explicitly listed are grouped
together, and full-width Latin is accepted). `<`, `>` and `(` outside quotes are
detected in a pre-pass, before any other local or domain check, so an input like
`a@exa<mple.com` reports `ErrDisplayName`.

**The "sendable" promise has a scope.** With the default configuration, an
address that passes validation is accepted by `net/mail.ParseAddress`
unchanged (this is covered by a property test and a fuzz test). `AllowQuotedLocal` and
`AllowIPLiteral` are **outside that promise**: addresses they admit are valid
per the RFC but many relays reject them. They are tested only for stability
and absence of panics.

## Errors

Failures are `*emailvalidator.Error` values carrying the original `Input`, the
byte offset `Pos` (`-1` if unknown), and a `Reason` sentinel. Use `errors.Is` to test for a reason and `errors.As` to get the details:

```go
err := emailvalidator.Validate("zhang san@example.com")

if errors.Is(err, emailvalidator.ErrLocalChar) {
	// reject with a specific message
}

var ve *emailvalidator.Error
if errors.As(err, &ve) {
	fmt.Println(ve.Reason, ve.Pos, ve.Input)
	// invalid character in local part 5 zhang san@example.com
}
```

| Sentinel | Meaning |
|---|---|
| `ErrEmpty` | empty address |
| `ErrWhitespace` | leading or trailing whitespace (`Validate` does not trim; `Normalize` does) |
| `ErrDisplayName` | display name or angle brackets |
| `ErrMissingAt` | no `@` |
| `ErrTooLong` | address over 254 octets |
| `ErrLocalEmpty` | empty local part |
| `ErrLocalTooLong` | local part over 64 octets |
| `ErrLocalChar` | invalid character in local part |
| `ErrNonASCIILocal` | non-ASCII local part without the `EAI` profile |
| `ErrDot` | leading, trailing or consecutive dot |
| `ErrQuotedLocal` | quoted local part not allowed |
| `ErrComment` | comments not allowed |
| `ErrDomainEmpty` | empty domain |
| `ErrDomainTooLong` | domain over 253 octets |
| `ErrLabel` | invalid domain label |
| `ErrLabelTooLong` | label over 63 octets |
| `ErrNonASCIIDomain` | non-ASCII domain without `IDN` or `EAI` |
| `ErrIDN` | invalid internationalized domain |
| `ErrNoTLD` | domain has no TLD |
| `ErrNumericTLD` | TLD is all digits |
| `ErrIPLiteral` | IP literal not allowed |
| `ErrMixedScript` | local part mixes scripts (`EAI`) |
| `ErrNotNFC` | local part not NFC-normalized (`EAI`) |

## API

```go
func Validate(addr string) error          // default configuration
func IsValid(addr string) bool

func New(opts ...Option) *Validator
func (v *Validator) Validate(addr string) error
func (v *Validator) Parse(addr string) (Address, error)
func (v *Validator) Normalize(addr string) (string, error)
```

## Non-goals

- MX lookups, SMTP probing, or checking that a mailbox exists.
- Disposable-address lists and typo correction.
- Parsing full RFC 5322 headers (display names, groups, address lists). Only a
  single addr-spec is handled.
- Provider-specific normalization (dots or `+tag` in particular providers).

## 中文说明

### 为什么默认不放行中文邮箱

中国大陆等地区确实存在中文邮箱（如 `用户@例子.中国`），但默认的 `ASCII` 档会拒绝
非 ASCII 的 local-part。原因是：发送含中文 local-part 的邮件需要 SMTPUTF8
（RFC 6531），要求从发信方到收信方的**整条中继链路**都支持，而现实中支持并不普遍，
许多 ESP 的 SMTP 接口就不支持。在不支持的通道上，SMTP 库会把地址按 RFC 2047
编码成 `=?UTF-8?q?...?=`，结果要么被拒收，要么在发信前的二次解析时报错
（`mail: no angle-addr`），重试也不会成功。

所以本库的默认值偏保守：只放行普通 SMTP 一定能发的地址。

- 通道不支持 SMTPUTF8，但域名是中文：用 `IDN` 档，并**务必使用 `Normalize` 的返回值发送**（域名会被转成 punycode）。
- 已确认整条链路都支持 SMTPUTF8：用 `EAI` 档，同样使用 `Normalize` 的结果；`EAI` 档默认做 NFC 检查和混合文字检测（拦截 `pаypal` 这类西里尔字母混入的冒充），`张三abc`、`用户123` 这类正常写法可以通过。
- `AllowQuotedLocal`、`AllowIPLiteral` 放行的地址不在"能发"的承诺范围内，请自行评估。

## License

MIT, see [LICENSE](LICENSE).
