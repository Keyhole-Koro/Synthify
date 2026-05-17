import React from 'react';

export function AuthForm() {
  return (
    <div className="flex flex-col gap-4 pt-1">
      {/* Heading */}
      <div>
        <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">
          Sign in
        </p>
        <h3 className="mt-0.5 text-lg font-semibold tracking-tight text-stone-800">
          おかえりなさい
        </h3>
        <p className="mt-0.5 text-xs text-stone-400">
          ドキュメントの知識ツリーを探索する
        </p>
      </div>
    </div>
  );
}
