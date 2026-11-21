import type { Metadata } from 'next';
import { LegalDocumentPage } from '@/features/legal/LegalDocumentPage';
import { PRIVACY_POLICY_EFFECTIVE_DATE } from '@/features/legal/operator';
import { PRIVACY_POLICY } from '@/features/legal/privacyPolicy';

export const metadata: Metadata = {
  title: 'プライバシーポリシー | Synthify',
};

export default function PrivacyPolicyPage() {
  return <LegalDocumentPage content={PRIVACY_POLICY} effectiveDate={PRIVACY_POLICY_EFFECTIVE_DATE} current="privacy" />;
}
