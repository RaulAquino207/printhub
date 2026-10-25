import { Suspense } from "react"
import type { Metadata } from "next"

import { PageHeader } from "@/components/page-header"
import { SignInForm } from "@/components/sign-in-form"

export const metadata: Metadata = {
  title: "Entrar",
}

export default function SignInPage() {
  return (
    <div className="mx-auto flex w-full max-w-sm flex-col gap-8 pt-4 sm:pt-12">
      <PageHeader
        title="Entrar"
        description="Use a conta criada pelo administrador do PrintHub."
      />
      {/* SignInForm reads ?next= with useSearchParams, which needs a
          Suspense boundary so the rest of the page can be prerendered. */}
      <Suspense>
        <SignInForm />
      </Suspense>
    </div>
  )
}
