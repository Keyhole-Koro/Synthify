import type { Metadata } from 'next';
import { LegalDocumentPage } from '@/features/legal/LegalDocumentPage';
import { TERMS_EFFECTIVE_DATE } from '@/features/legal/operator';
import { TERMS_OF_SERVICE } from '@/features/legal/termsOfService';

export const metadata: Metadata = {
  title: '利用規約 | Synthify',
};

export default function TermsOfServicePage() {
  return <LegalDocumentPage content={TERMS_OF_SERVICE} effectiveDate={TERMS_EFFECTIVE_DATE} current="terms" />;
}
