/**
 * Star Rating Component
 */

import { Star } from 'lucide-react';

interface StarRatingProps {
  rating: number;
  size?: 'sm' | 'md' | 'lg';
  showValue?: boolean;
  reviewCount?: number;
}

export function StarRating({
  rating,
  size = 'md',
  showValue = false,
  reviewCount,
}: StarRatingProps) {
  const sizes = {
    sm: 'w-3 h-3',
    md: 'w-4 h-4',
    lg: 'w-5 h-5',
  };

  const textSizes = {
    sm: 'text-xs',
    md: 'text-sm',
    lg: 'text-base',
  };

  return (
    <div className="flex items-center">
      <div className="flex items-center space-x-0.5">
        {[1, 2, 3, 4, 5].map((star) => (
          <Star
            key={star}
            className={`${sizes[size]} ${
              star <= Math.round(rating)
                ? 'text-yellow-400 fill-yellow-400'
                : 'text-dark-600'
            }`}
          />
        ))}
      </div>
      {showValue && (
        <span className={`ml-2 text-dark-300 ${textSizes[size]}`}>
          {rating.toFixed(1)}
        </span>
      )}
      {reviewCount !== undefined && (
        <span className={`ml-1 text-dark-500 ${textSizes[size]}`}>
          ({reviewCount})
        </span>
      )}
    </div>
  );
}

export default StarRating;
