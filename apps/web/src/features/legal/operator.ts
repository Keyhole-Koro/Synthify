/**
 * Facts about the operator that the terms and the privacy policy print.
 *
 * None of these are recorded anywhere in the repository, so they start as null and the
 * pages show 【要確認：…】 in their place. Fill every field (and the court and the dates
 * below) before the pages go live; the privacy policy must name the operator, its address
 * and, for a company, its representative (個人情報保護法 第32条).
 */
export const OPERATOR = {
  /** The operator's legal name: a company name, or an individual's name or trade name. */
  name: null as string | null,
  /** The address (a company's head office). */
  address: null as string | null,
  /** The representative's name, for a company. */
  representative: null as string | null,
  /** Where users send questions and requests about their data. */
  contactEmail: null as string | null,
};

/** The court of first instance with exclusive jurisdiction (利用規約「準拠法及び管轄裁判所」). */
export const COURT: string | null = null;

/** The date each document takes effect, as YYYY-MM-DD. */
export const TERMS_EFFECTIVE_DATE: string | null = null;
export const PRIVACY_POLICY_EFFECTIVE_DATE: string | null = null;
