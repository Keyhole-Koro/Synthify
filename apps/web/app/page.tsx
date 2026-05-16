'use client';

import { LandingPageView } from '@/features/landing/LandingPageView';
import { useLandingPageController } from '@/features/landing/useLandingPageController';

export default function LandingPage() {
  const controller = useLandingPageController();
  return <LandingPageView controller={controller} />;
}
