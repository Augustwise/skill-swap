export type DashboardMatch = {
  id: string;
  name: string;
  initials: string;
  color: string;
  university: string;
  score: number;
  rating: string;
  reviews: string;
  teaching: readonly string[];
  learning: readonly string[];
};

export type DashboardExchange = {
  name: string;
  initials: string;
  color: string;
  learning: string;
  completed: number;
  total: number;
  time: string;
};

export type DashboardRequest = {
  name: string;
  initials: string;
  color: string;
  time: string;
  skill: string;
};

// Design fixtures only. The live dashboard stays empty until activity APIs are available.
export const demoMatches = [
  {
    id: "olena",
    name: "Олена Ковальчук",
    initials: "ОК",
    color: "purple",
    university: "КПІ · ФІОТ, 3 курс",
    score: 94,
    rating: "4.9",
    reviews: "23 відгуки",
    teaching: ["Photoshop", "Figma"],
    learning: ["Гітара", "Сольфеджіо"],
  },
  {
    id: "dmytro",
    name: "Дмитро Савчук",
    initials: "ДС",
    color: "blue",
    university: "КНУ · Філологія, 2 курс",
    score: 88,
    rating: "4.7",
    reviews: "14 відгуків",
    teaching: ["Англійська B2", "IELTS"],
    learning: ["Гітара", "Укулеле"],
  },
] as const;

export const demoExchanges = [
  {
    name: "Марія Гнатюк",
    initials: "МГ",
    color: "pink",
    learning: "Марія: Web-дизайн",
    completed: 3,
    total: 6,
    time: "Сьогодні, 18:00",
  },
  {
    name: "Іван Петренко",
    initials: "ІП",
    color: "green",
    learning: "Іван: Python",
    completed: 1,
    total: 4,
    time: "Пт, 15 бер, 17:00",
  },
] as const;

export const demoRequests = [
  {
    name: "Софія Ткаченко",
    initials: "СТ",
    color: "amber",
    time: "2 год тому",
    skill: "Ілюстрація",
  },
  { name: "Олег Бондар", initials: "ОБ", color: "indigo", time: "вчора", skill: "Python" },
] as const;
