import React from 'react';

export const LoadingScreen: React.FC = () => {
  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center">
      <div className="flex flex-col items-center gap-4">
        {/* Animated logo */}
        <div className="relative">
          <div className="w-16 h-16 rounded-full border-4 border-omniphi-600/20 border-t-omniphi-500 animate-spin" />
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="text-2xl font-bold text-gradient">O</span>
          </div>
        </div>
        <p className="text-dark-400 text-sm">Loading...</p>
      </div>
    </div>
  );
};
