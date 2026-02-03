package service

// getBaseDesignHTML returns the HTML template for a base design
func getBaseDesignHTML(design string) string {
	switch design {
	case "minimal":
		return minimalDesignHTML
	case "modern":
		return modernDesignHTML
	default:
		return professionalDesignHTML
	}
}

// Professional design — Blue header bar, structured table, corporate
const professionalDesignHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: {{if .Branding}}{{if .Branding.FontFamily}}{{.Branding.FontFamily}}{{else}}Helvetica, Arial, sans-serif{{end}}{{else}}Helvetica, Arial, sans-serif{{end}};
    font-size: 12px;
    color: #333;
    line-height: 1.5;
  }
  .header {
    background: {{if .Branding}}{{if .Branding.PrimaryColor}}{{.Branding.PrimaryColor}}{{else}}#2563eb{{end}}{{else}}#2563eb{{end}};
    color: white;
    padding: 30px 40px;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
  }
  .header-left h1 { font-size: 24px; margin-bottom: 4px; }
  .header-left p { font-size: 11px; opacity: 0.9; }
  .header-right { text-align: right; }
  .header-right h2 { font-size: 28px; font-weight: 300; margin-bottom: 4px; }
  .header-right p { font-size: 11px; opacity: 0.9; }
  .content { padding: 30px 40px; }
  .meta-row { display: flex; justify-content: space-between; margin-bottom: 30px; }
  .meta-block { flex: 1; }
  .meta-block h3 {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: #888;
    margin-bottom: 6px;
  }
  .meta-block p { font-size: 12px; margin-bottom: 2px; }
  table { width: 100%; border-collapse: collapse; margin-bottom: 30px; }
  table th {
    background: #f8fafc;
    border-bottom: 2px solid {{if .Branding}}{{if .Branding.PrimaryColor}}{{.Branding.PrimaryColor}}{{else}}#2563eb{{end}}{{else}}#2563eb{{end}};
    padding: 10px 12px;
    text-align: left;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: #555;
  }
  table th.right, table td.right { text-align: right; }
  table td { padding: 10px 12px; border-bottom: 1px solid #eee; }
  table td.desc { color: #888; font-size: 11px; }
  .totals { display: flex; justify-content: flex-end; margin-bottom: 30px; }
  .totals-table { width: 280px; }
  .totals-table tr td { padding: 6px 0; }
  .totals-table tr td:last-child { text-align: right; font-weight: 500; }
  .totals-table .grand-total td {
    border-top: 2px solid #333;
    padding-top: 10px;
    font-size: 16px;
    font-weight: 700;
  }
  .section { margin-bottom: 20px; }
  .section h3 {
    font-size: 13px;
    font-weight: 600;
    color: {{if .Branding}}{{if .Branding.PrimaryColor}}{{.Branding.PrimaryColor}}{{else}}#2563eb{{end}}{{else}}#2563eb{{end}};
    margin-bottom: 8px;
    padding-bottom: 4px;
    border-bottom: 1px solid #eee;
  }
  .section p { font-size: 12px; white-space: pre-wrap; }
  .footer {
    margin-top: 40px;
    padding-top: 15px;
    border-top: 1px solid #ddd;
    text-align: center;
    font-size: 10px;
    color: #aaa;
  }
  .status-badge {
    display: inline-block;
    padding: 3px 10px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
    background: rgba(255,255,255,0.2);
  }
</style>
</head>
<body>
{{if sectionEnabled "header"}}
<div class="header">
  <div class="header-left">
    {{if .HasLogo}}<img src="{{.Branding.LogoURL}}" alt="Logo" style="max-height:50px;margin-bottom:8px;">{{end}}
    {{if fieldEnabled "header" "companyName"}}
    <h1>{{if .Branding}}{{if .Branding.CompanyName}}{{.Branding.CompanyName}}{{end}}{{end}}</h1>
    {{end}}
  </div>
  <div class="header-right">
    <h2>QUOTE</h2>
    {{if fieldEnabled "header" "quoteNumber"}}<p><strong>{{.Quote.QuoteNumber}}</strong></p>{{end}}
    {{if fieldEnabled "header" "quoteDate"}}<p>Date: {{.FormattedCreatedAt}}</p>{{end}}
    {{if fieldEnabled "header" "validUntil"}}{{if .HasValidUntil}}<p>Valid Until: {{.FormattedValidUntil}}</p>{{end}}{{end}}
    {{if fieldEnabled "header" "status"}}<span class="status-badge">{{.Quote.Status}}</span>{{end}}
  </div>
</div>
{{end}}

<div class="content">
  <div class="meta-row">
    {{if sectionEnabled "billTo"}}
    <div class="meta-block">
      <h3>Bill To</h3>
      {{if fieldEnabled "billTo" "contactName"}}{{if .HasContactName}}<p><strong>{{.Quote.ContactName}}</strong></p>{{end}}{{end}}
      {{if fieldEnabled "billTo" "accountName"}}{{if .HasAccountName}}<p>{{.Quote.AccountName}}</p>{{end}}{{end}}
      {{if fieldEnabled "billTo" "billingAddressStreet"}}{{if .Quote.BillingAddressStreet}}<p>{{.Quote.BillingAddressStreet}}</p>{{end}}{{end}}
      {{if fieldEnabled "billTo" "billingAddressCity"}}
        {{if .Quote.BillingAddressCity}}
        <p>{{.Quote.BillingAddressCity}}{{if .Quote.BillingAddressState}}, {{.Quote.BillingAddressState}}{{end}} {{.Quote.BillingAddressPostalCode}}</p>
        {{end}}
      {{end}}
      {{if fieldEnabled "billTo" "billingAddressCountry"}}{{if .Quote.BillingAddressCountry}}<p>{{.Quote.BillingAddressCountry}}</p>{{end}}{{end}}
    </div>
    {{end}}

    {{if sectionEnabled "shipTo"}}
    {{if .HasShippingAddress}}
    <div class="meta-block">
      <h3>Ship To</h3>
      {{if .Quote.ShippingAddressStreet}}<p>{{.Quote.ShippingAddressStreet}}</p>{{end}}
      {{if .Quote.ShippingAddressCity}}
      <p>{{.Quote.ShippingAddressCity}}{{if .Quote.ShippingAddressState}}, {{.Quote.ShippingAddressState}}{{end}} {{.Quote.ShippingAddressPostalCode}}</p>
      {{end}}
      {{if .Quote.ShippingAddressCountry}}<p>{{.Quote.ShippingAddressCountry}}</p>{{end}}
    </div>
    {{end}}
    {{end}}
  </div>

  {{if sectionEnabled "description"}}
  {{if .HasDescription}}
  <div class="section">
    <h3>Description</h3>
    <p>{{.Quote.Description}}</p>
  </div>
  {{end}}
  {{end}}

  {{if sectionEnabled "lineItems"}}
  <table>
    <thead>
      <tr>
        <th style="width:5%">#</th>
        {{if hasLineItemField "name"}}<th>Item</th>{{end}}
        {{if hasLineItemField "sku"}}<th>SKU</th>{{end}}
        {{if hasLineItemField "quantity"}}<th class="right">Qty</th>{{end}}
        {{if hasLineItemField "unitPrice"}}<th class="right">Unit Price</th>{{end}}
        {{if hasLineItemField "discount"}}<th class="right">Discount</th>{{end}}
        {{if hasLineItemField "total"}}<th class="right">Total</th>{{end}}
      </tr>
    </thead>
    <tbody>
      {{range $i, $item := .LineItems}}
      <tr>
        <td>{{$idx := $i}}{{$idx}}</td>
        {{if hasLineItemField "name"}}<td><strong>{{$item.Name}}</strong>{{if $item.Description}}<br><span class="desc">{{$item.Description}}</span>{{end}}</td>{{end}}
        {{if hasLineItemField "sku"}}<td>{{$item.SKU}}</td>{{end}}
        {{if hasLineItemField "quantity"}}<td class="right">{{printf "%.0f" $item.Quantity}}</td>{{end}}
        {{if hasLineItemField "unitPrice"}}<td class="right">{{formatCurrency $item.UnitPrice $.Quote.Currency}}</td>{{end}}
        {{if hasLineItemField "discount"}}<td class="right">{{if gt $item.DiscountPercent 0.0}}{{printf "%.0f" $item.DiscountPercent}}%{{else}}-{{end}}</td>{{end}}
        {{if hasLineItemField "total"}}<td class="right">{{formatCurrency $item.Total $.Quote.Currency}}</td>{{end}}
      </tr>
      {{end}}
    </tbody>
  </table>
  {{end}}

  {{if sectionEnabled "totals"}}
  <div class="totals">
    <table class="totals-table">
      {{if fieldEnabled "totals" "subtotal"}}<tr><td>Subtotal</td><td>{{.FormattedSubtotal}}</td></tr>{{end}}
      {{if fieldEnabled "totals" "discountAmount"}}{{if .HasDiscount}}<tr><td>Discount{{if gt .Quote.DiscountPercent 0.0}} ({{printf "%.0f" .Quote.DiscountPercent}}%){{end}}</td><td>-{{.FormattedDiscountAmount}}</td></tr>{{end}}{{end}}
      {{if fieldEnabled "totals" "taxAmount"}}{{if .HasTax}}<tr><td>Tax{{if gt .Quote.TaxPercent 0.0}} ({{printf "%.1f" .Quote.TaxPercent}}%){{end}}</td><td>{{.FormattedTaxAmount}}</td></tr>{{end}}{{end}}
      {{if fieldEnabled "totals" "shippingAmount"}}{{if .HasShipping}}<tr><td>Shipping</td><td>{{.FormattedShippingAmount}}</td></tr>{{end}}{{end}}
      {{if fieldEnabled "totals" "grandTotal"}}<tr class="grand-total"><td>Total</td><td>{{.FormattedGrandTotal}}</td></tr>{{end}}
    </table>
  </div>
  {{end}}

  {{if sectionEnabled "terms"}}
  {{if .HasTerms}}
  <div class="section">
    <h3>Terms &amp; Conditions</h3>
    <p>{{.Quote.Terms}}</p>
  </div>
  {{end}}
  {{end}}

  {{if sectionEnabled "notes"}}
  {{if .HasNotes}}
  <div class="section">
    <h3>Notes</h3>
    <p>{{.Quote.Notes}}</p>
  </div>
  {{end}}
  {{end}}

  {{if sectionEnabled "footer"}}
  <div class="footer">
    {{if fieldEnabled "footer" "companyName"}}
    <p>{{if .Branding}}{{if .Branding.CompanyName}}{{.Branding.CompanyName}}{{end}}{{end}}</p>
    {{end}}
    {{if fieldEnabled "footer" "generatedDate"}}
    <p>Generated on {{.FormattedCreatedAt}}</p>
    {{end}}
  </div>
  {{end}}
</div>
</body>
</html>`

// Minimal design — Clean white, thin borders, modern typography
const minimalDesignHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: {{if .Branding}}{{if .Branding.FontFamily}}{{.Branding.FontFamily}}{{else}}'Segoe UI', system-ui, sans-serif{{end}}{{else}}'Segoe UI', system-ui, sans-serif{{end}};
    font-size: 12px;
    color: #1a1a1a;
    line-height: 1.6;
    padding: 40px;
  }
  .header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 40px; padding-bottom: 20px; border-bottom: 1px solid #e5e5e5; }
  .header-left h1 { font-size: 14px; font-weight: 600; color: #666; text-transform: uppercase; letter-spacing: 2px; }
  .header-left .company { font-size: 20px; font-weight: 700; color: #1a1a1a; margin-top: 4px; }
  .header-right { text-align: right; }
  .header-right .quote-label { font-size: 11px; color: #999; text-transform: uppercase; letter-spacing: 1px; }
  .header-right .quote-number { font-size: 18px; font-weight: 600; margin: 2px 0; }
  .header-right .meta { font-size: 11px; color: #666; }
  .addresses { display: flex; gap: 40px; margin-bottom: 30px; }
  .address-block h3 { font-size: 10px; text-transform: uppercase; letter-spacing: 1px; color: #999; margin-bottom: 8px; }
  .address-block p { font-size: 12px; color: #333; margin-bottom: 2px; }
  table { width: 100%; border-collapse: collapse; margin-bottom: 30px; }
  table th { padding: 8px 10px; text-align: left; font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; color: #999; border-bottom: 1px solid #e5e5e5; font-weight: 500; }
  table th.right, table td.right { text-align: right; }
  table td { padding: 10px; border-bottom: 1px solid #f0f0f0; font-size: 12px; }
  .totals-wrap { display: flex; justify-content: flex-end; }
  .totals-table { width: 260px; }
  .totals-table td { padding: 5px 0; font-size: 12px; }
  .totals-table td:last-child { text-align: right; }
  .totals-table .grand-total td { border-top: 1px solid #333; padding-top: 10px; font-size: 15px; font-weight: 700; }
  .section { margin-bottom: 20px; }
  .section h3 { font-size: 10px; text-transform: uppercase; letter-spacing: 1px; color: #999; margin-bottom: 6px; }
  .section p { font-size: 12px; white-space: pre-wrap; color: #555; }
  .footer { margin-top: 40px; text-align: center; font-size: 10px; color: #bbb; }
</style>
</head>
<body>
{{if sectionEnabled "header"}}
<div class="header">
  <div class="header-left">
    {{if .HasLogo}}<img src="{{.Branding.LogoURL}}" alt="Logo" style="max-height:40px;margin-bottom:6px;">{{end}}
    <h1>Quote</h1>
    {{if fieldEnabled "header" "companyName"}}<div class="company">{{if .Branding}}{{.Branding.CompanyName}}{{end}}</div>{{end}}
  </div>
  <div class="header-right">
    {{if fieldEnabled "header" "quoteNumber"}}<div class="quote-label">Quote No.</div><div class="quote-number">{{.Quote.QuoteNumber}}</div>{{end}}
    {{if fieldEnabled "header" "quoteDate"}}<div class="meta">{{.FormattedCreatedAt}}</div>{{end}}
    {{if fieldEnabled "header" "validUntil"}}{{if .HasValidUntil}}<div class="meta">Valid until {{.FormattedValidUntil}}</div>{{end}}{{end}}
  </div>
</div>
{{end}}

<div class="addresses">
  {{if sectionEnabled "billTo"}}
  <div class="address-block">
    <h3>Bill To</h3>
    {{if .HasContactName}}<p><strong>{{.Quote.ContactName}}</strong></p>{{end}}
    {{if .HasAccountName}}<p>{{.Quote.AccountName}}</p>{{end}}
    {{if .Quote.BillingAddressStreet}}<p>{{.Quote.BillingAddressStreet}}</p>{{end}}
    {{if .Quote.BillingAddressCity}}<p>{{.Quote.BillingAddressCity}}{{if .Quote.BillingAddressState}}, {{.Quote.BillingAddressState}}{{end}} {{.Quote.BillingAddressPostalCode}}</p>{{end}}
    {{if .Quote.BillingAddressCountry}}<p>{{.Quote.BillingAddressCountry}}</p>{{end}}
  </div>
  {{end}}
  {{if sectionEnabled "shipTo"}}{{if .HasShippingAddress}}
  <div class="address-block">
    <h3>Ship To</h3>
    {{if .Quote.ShippingAddressStreet}}<p>{{.Quote.ShippingAddressStreet}}</p>{{end}}
    {{if .Quote.ShippingAddressCity}}<p>{{.Quote.ShippingAddressCity}}{{if .Quote.ShippingAddressState}}, {{.Quote.ShippingAddressState}}{{end}} {{.Quote.ShippingAddressPostalCode}}</p>{{end}}
    {{if .Quote.ShippingAddressCountry}}<p>{{.Quote.ShippingAddressCountry}}</p>{{end}}
  </div>
  {{end}}{{end}}
</div>

{{if sectionEnabled "description"}}{{if .HasDescription}}
<div class="section"><h3>Description</h3><p>{{.Quote.Description}}</p></div>
{{end}}{{end}}

{{if sectionEnabled "lineItems"}}
<table>
  <thead><tr><th>#</th><th>Item</th><th class="right">Qty</th><th class="right">Price</th><th class="right">Total</th></tr></thead>
  <tbody>
    {{range $i, $item := .LineItems}}
    <tr>
      <td>{{$i}}</td>
      <td>{{$item.Name}}{{if $item.Description}}<br><span style="color:#999;font-size:11px">{{$item.Description}}</span>{{end}}</td>
      <td class="right">{{printf "%.0f" $item.Quantity}}</td>
      <td class="right">{{formatCurrency $item.UnitPrice $.Quote.Currency}}</td>
      <td class="right">{{formatCurrency $item.Total $.Quote.Currency}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
{{end}}

{{if sectionEnabled "totals"}}
<div class="totals-wrap">
  <table class="totals-table">
    <tr><td>Subtotal</td><td>{{.FormattedSubtotal}}</td></tr>
    {{if .HasDiscount}}<tr><td>Discount</td><td>-{{.FormattedDiscountAmount}}</td></tr>{{end}}
    {{if .HasTax}}<tr><td>Tax</td><td>{{.FormattedTaxAmount}}</td></tr>{{end}}
    {{if .HasShipping}}<tr><td>Shipping</td><td>{{.FormattedShippingAmount}}</td></tr>{{end}}
    <tr class="grand-total"><td>Total</td><td>{{.FormattedGrandTotal}}</td></tr>
  </table>
</div>
{{end}}

{{if sectionEnabled "terms"}}{{if .HasTerms}}<div class="section"><h3>Terms &amp; Conditions</h3><p>{{.Quote.Terms}}</p></div>{{end}}{{end}}
{{if sectionEnabled "notes"}}{{if .HasNotes}}<div class="section"><h3>Notes</h3><p>{{.Quote.Notes}}</p></div>{{end}}{{end}}

{{if sectionEnabled "footer"}}
<div class="footer">
  {{if .Branding}}{{if .Branding.CompanyName}}<p>{{.Branding.CompanyName}}</p>{{end}}{{end}}
</div>
{{end}}
</body>
</html>`

// Modern design — Gradient accents, rounded cards, contemporary
const modernDesignHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: {{if .Branding}}{{if .Branding.FontFamily}}{{.Branding.FontFamily}}{{else}}'Inter', system-ui, sans-serif{{end}}{{else}}'Inter', system-ui, sans-serif{{end}};
    font-size: 12px;
    color: #1f2937;
    line-height: 1.6;
    background: #f9fafb;
  }
  .page { background: white; margin: 0 auto; padding: 0; }
  .header-bar {
    background: linear-gradient(135deg,
      {{if .Branding}}{{if .Branding.PrimaryColor}}{{.Branding.PrimaryColor}}{{else}}#6366f1{{end}}{{else}}#6366f1{{end}},
      {{if .Branding}}{{if .Branding.AccentColor}}{{.Branding.AccentColor}}{{else}}#8b5cf6{{end}}{{else}}#8b5cf6{{end}}
    );
    color: white;
    padding: 30px 40px;
    border-radius: 0 0 16px 16px;
  }
  .header-inner { display: flex; justify-content: space-between; align-items: center; }
  .header-left .company { font-size: 22px; font-weight: 700; }
  .header-right { text-align: right; }
  .header-right .label { font-size: 32px; font-weight: 200; opacity: 0.9; }
  .header-right .number { font-size: 13px; opacity: 0.8; margin-top: 2px; }
  .content { padding: 30px 40px; }
  .info-cards { display: flex; gap: 20px; margin-bottom: 30px; }
  .info-card {
    flex: 1;
    background: #f9fafb;
    border-radius: 10px;
    padding: 16px 20px;
    border: 1px solid #e5e7eb;
  }
  .info-card h4 { font-size: 10px; text-transform: uppercase; letter-spacing: 1px; color: #9ca3af; margin-bottom: 8px; font-weight: 500; }
  .info-card p { font-size: 12px; margin-bottom: 2px; }
  .info-card .value { font-weight: 600; font-size: 13px; }
  table { width: 100%; border-collapse: separate; border-spacing: 0; margin-bottom: 30px; border-radius: 10px; overflow: hidden; border: 1px solid #e5e7eb; }
  table th {
    background: #f3f4f6;
    padding: 12px 14px;
    text-align: left;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: #6b7280;
    font-weight: 600;
  }
  table th.right, table td.right { text-align: right; }
  table td { padding: 12px 14px; border-top: 1px solid #f3f4f6; }
  table tbody tr:hover { background: #fafbfc; }
  .totals-wrap { display: flex; justify-content: flex-end; margin-bottom: 30px; }
  .totals-card {
    background: #f9fafb;
    border-radius: 10px;
    padding: 20px;
    width: 280px;
    border: 1px solid #e5e7eb;
  }
  .totals-card .row { display: flex; justify-content: space-between; padding: 6px 0; font-size: 12px; }
  .totals-card .grand-total { border-top: 2px solid #1f2937; margin-top: 8px; padding-top: 10px; font-size: 16px; font-weight: 700; }
  .section { margin-bottom: 20px; background: #f9fafb; border-radius: 10px; padding: 16px 20px; border: 1px solid #e5e7eb; }
  .section h3 {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: {{if .Branding}}{{if .Branding.PrimaryColor}}{{.Branding.PrimaryColor}}{{else}}#6366f1{{end}}{{else}}#6366f1{{end}};
    margin-bottom: 8px;
    font-weight: 600;
  }
  .section p { font-size: 12px; white-space: pre-wrap; color: #4b5563; }
  .footer { text-align: center; padding: 20px; font-size: 10px; color: #9ca3af; }
  .status-pill {
    display: inline-block;
    padding: 4px 12px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    background: rgba(255,255,255,0.2);
    margin-top: 6px;
  }
</style>
</head>
<body>
<div class="page">
{{if sectionEnabled "header"}}
<div class="header-bar">
  <div class="header-inner">
    <div class="header-left">
      {{if .HasLogo}}<img src="{{.Branding.LogoURL}}" alt="Logo" style="max-height:45px;margin-bottom:8px;border-radius:6px;">{{end}}
      {{if fieldEnabled "header" "companyName"}}<div class="company">{{if .Branding}}{{.Branding.CompanyName}}{{end}}</div>{{end}}
    </div>
    <div class="header-right">
      <div class="label">Quote</div>
      {{if fieldEnabled "header" "quoteNumber"}}<div class="number">{{.Quote.QuoteNumber}}</div>{{end}}
      {{if fieldEnabled "header" "status"}}<div class="status-pill">{{.Quote.Status}}</div>{{end}}
    </div>
  </div>
</div>
{{end}}

<div class="content">
  <div class="info-cards">
    {{if sectionEnabled "header"}}
    <div class="info-card">
      <h4>Quote Details</h4>
      {{if fieldEnabled "header" "quoteDate"}}<p>Date: <span class="value">{{.FormattedCreatedAt}}</span></p>{{end}}
      {{if fieldEnabled "header" "validUntil"}}{{if .HasValidUntil}}<p>Valid Until: <span class="value">{{.FormattedValidUntil}}</span></p>{{end}}{{end}}
      <p>Currency: <span class="value">{{.Quote.Currency}}</span></p>
    </div>
    {{end}}

    {{if sectionEnabled "billTo"}}
    <div class="info-card">
      <h4>Bill To</h4>
      {{if .HasContactName}}<p class="value">{{.Quote.ContactName}}</p>{{end}}
      {{if .HasAccountName}}<p>{{.Quote.AccountName}}</p>{{end}}
      {{if .Quote.BillingAddressStreet}}<p>{{.Quote.BillingAddressStreet}}</p>{{end}}
      {{if .Quote.BillingAddressCity}}<p>{{.Quote.BillingAddressCity}}{{if .Quote.BillingAddressState}}, {{.Quote.BillingAddressState}}{{end}} {{.Quote.BillingAddressPostalCode}}</p>{{end}}
    </div>
    {{end}}

    {{if sectionEnabled "shipTo"}}{{if .HasShippingAddress}}
    <div class="info-card">
      <h4>Ship To</h4>
      {{if .Quote.ShippingAddressStreet}}<p>{{.Quote.ShippingAddressStreet}}</p>{{end}}
      {{if .Quote.ShippingAddressCity}}<p>{{.Quote.ShippingAddressCity}}{{if .Quote.ShippingAddressState}}, {{.Quote.ShippingAddressState}}{{end}} {{.Quote.ShippingAddressPostalCode}}</p>{{end}}
    </div>
    {{end}}{{end}}
  </div>

  {{if sectionEnabled "description"}}{{if .HasDescription}}
  <div class="section"><h3>Description</h3><p>{{.Quote.Description}}</p></div>
  {{end}}{{end}}

  {{if sectionEnabled "lineItems"}}
  <table>
    <thead><tr><th>#</th><th>Item</th><th class="right">Qty</th><th class="right">Price</th><th class="right">Total</th></tr></thead>
    <tbody>
      {{range $i, $item := .LineItems}}
      <tr>
        <td>{{$i}}</td>
        <td><strong>{{$item.Name}}</strong>{{if $item.Description}}<br><span style="color:#9ca3af;font-size:11px">{{$item.Description}}</span>{{end}}</td>
        <td class="right">{{printf "%.0f" $item.Quantity}}</td>
        <td class="right">{{formatCurrency $item.UnitPrice $.Quote.Currency}}</td>
        <td class="right"><strong>{{formatCurrency $item.Total $.Quote.Currency}}</strong></td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{end}}

  {{if sectionEnabled "totals"}}
  <div class="totals-wrap">
    <div class="totals-card">
      <div class="row"><span>Subtotal</span><span>{{.FormattedSubtotal}}</span></div>
      {{if .HasDiscount}}<div class="row"><span>Discount</span><span>-{{.FormattedDiscountAmount}}</span></div>{{end}}
      {{if .HasTax}}<div class="row"><span>Tax</span><span>{{.FormattedTaxAmount}}</span></div>{{end}}
      {{if .HasShipping}}<div class="row"><span>Shipping</span><span>{{.FormattedShippingAmount}}</span></div>{{end}}
      <div class="row grand-total"><span>Total</span><span>{{.FormattedGrandTotal}}</span></div>
    </div>
  </div>
  {{end}}

  {{if sectionEnabled "terms"}}{{if .HasTerms}}<div class="section"><h3>Terms &amp; Conditions</h3><p>{{.Quote.Terms}}</p></div>{{end}}{{end}}
  {{if sectionEnabled "notes"}}{{if .HasNotes}}<div class="section"><h3>Notes</h3><p>{{.Quote.Notes}}</p></div>{{end}}{{end}}

  {{if sectionEnabled "footer"}}
  <div class="footer">
    {{if .Branding}}{{if .Branding.CompanyName}}<p>{{.Branding.CompanyName}}</p>{{end}}{{end}}
  </div>
  {{end}}
</div>
</div>
</body>
</html>`
